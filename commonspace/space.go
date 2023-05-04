package commonspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/headsync"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/acl/syncacl"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/settings"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/metric"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/anytypeio/any-sync/util/multiqueue"
	"github.com/anytypeio/any-sync/util/slice"
	"github.com/cheggaaa/mb/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSpaceClosed = errors.New("space is closed")
)

type SpaceCreatePayload struct {
	// SigningKey is the signing key of the owner
	SigningKey crypto.PrivKey
	// SpaceType is an arbitrary string
	SpaceType string
	// ReadKey is a first symmetric encryption key for a space
	ReadKey []byte
	// ReplicationKey is a key which is to be used to determine the node where the space should be held
	ReplicationKey uint64
	// SpacePayload is an arbitrary payload related to space type
	SpacePayload []byte
	// MasterKey is the master key of the owner
	MasterKey crypto.PrivKey
}

type HandleMessage struct {
	Id                uint64
	ReceiveTime       time.Time
	StartHandlingTime time.Time
	Deadline          time.Time
	SenderId          string
	Message           *spacesyncproto.ObjectSyncMessage
	PeerCtx           context.Context
}

func (m HandleMessage) LogFields(fields ...zap.Field) []zap.Field {
	return append(fields,
		metric.SpaceId(m.Message.SpaceId),
		metric.ObjectId(m.Message.ObjectId),
		metric.QueueDur(m.StartHandlingTime.Sub(m.ReceiveTime)),
		metric.TotalDur(time.Since(m.ReceiveTime)),
	)
}

type SpaceDerivePayload struct {
	SigningKey   crypto.PrivKey
	MasterKey    crypto.PrivKey
	SpaceType    string
	SpacePayload []byte
}

type SpaceDescription struct {
	SpaceHeader          *spacesyncproto.RawSpaceHeaderWithId
	AclId                string
	AclPayload           []byte
	SpaceSettingsId      string
	SpaceSettingsPayload []byte
}

func NewSpaceId(id string, repKey uint64) string {
	return fmt.Sprintf("%s.%s", id, strconv.FormatUint(repKey, 36))
}

type Space interface {
	Id() string
	Init(ctx context.Context) error

	StoredIds() []string
	DebugAllHeads() []headsync.TreeHeads
	Description() (SpaceDescription, error)

	DeriveTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error)
	CreateTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error)
	PutTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, listener updatelistener.UpdateListener) (t objecttree.ObjectTree, err error)
	BuildTree(ctx context.Context, id string, opts BuildTreeOpts) (t objecttree.ObjectTree, err error)
	DeleteTree(ctx context.Context, id string) (err error)
	BuildHistoryTree(ctx context.Context, id string, opts HistoryTreeOpts) (t objecttree.HistoryTree, err error)

	SpaceDeleteRawChange(ctx context.Context) (raw *treechangeproto.RawTreeChangeWithId, err error)
	DeleteSpace(ctx context.Context, deleteChange *treechangeproto.RawTreeChangeWithId) (err error)

	HeadSync() headsync.HeadSync
	ObjectSync() objectsync.ObjectSync
	SyncStatus() syncstatus.StatusUpdater
	Storage() spacestorage.SpaceStorage

	HandleMessage(ctx context.Context, msg HandleMessage) (err error)

	TryClose(objectTTL time.Duration) (close bool, err error)
	Close() error
}

type space struct {
	id     string
	mu     sync.RWMutex
	header *spacesyncproto.RawSpaceHeaderWithId

	objectSync     objectsync.ObjectSync
	headSync       headsync.HeadSync
	syncStatus     syncstatus.StatusUpdater
	storage        spacestorage.SpaceStorage
	cache          *commonGetter
	account        accountservice.Service
	aclList        *syncacl.SyncAcl
	configuration  nodeconf.NodeConf
	settingsObject settings.SettingsObject
	peerManager    peermanager.PeerManager
	metric         metric.Metric

	handleQueue multiqueue.MultiQueue[HandleMessage]

	isClosed  *atomic.Bool
	isDeleted *atomic.Bool
	treesUsed *atomic.Int32
}

func (s *space) Id() string {
	return s.id
}

func (s *space) Description() (desc SpaceDescription, err error) {
	root := s.aclList.Root()
	settingsStorage, err := s.storage.TreeStorage(s.storage.SpaceSettingsId())
	if err != nil {
		return
	}
	settingsRoot, err := settingsStorage.Root()
	if err != nil {
		return
	}

	desc = SpaceDescription{
		SpaceHeader:          s.header,
		AclId:                root.Id,
		AclPayload:           root.Payload,
		SpaceSettingsId:      settingsRoot.Id,
		SpaceSettingsPayload: settingsRoot.RawChange,
	}
	return
}

func (s *space) Init(ctx context.Context) (err error) {
	log.With(zap.String("spaceId", s.id)).Debug("initializing space")
	s.storage = newCommonStorage(s.storage)

	header, err := s.storage.SpaceHeader()
	if err != nil {
		return
	}
	s.header = header
	initialIds, err := s.storage.StoredIds()
	if err != nil {
		return
	}
	aclStorage, err := s.storage.AclStorage()
	if err != nil {
		return
	}
	aclList, err := list.BuildAclListWithIdentity(s.account.Account(), aclStorage)
	if err != nil {
		return
	}
	s.aclList = syncacl.NewSyncAcl(aclList, s.objectSync.MessagePool())
	s.cache.AddObject(s.aclList)

	deletionState := settingsstate.NewObjectDeletionState(s.storage)
	deps := settings.Deps{
		BuildFunc: func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error) {
			res, err := s.BuildTree(ctx, id, BuildTreeOpts{
				Listener:           listener,
				WaitTreeRemoteSync: false,
			})
			log.Debug("building settings tree", zap.String("id", id), zap.String("spaceId", s.id))
			if err != nil {
				return
			}
			t = res.(synctree.SyncTree)
			return
		},
		Account:       s.account,
		TreeGetter:    s.cache,
		Store:         s.storage,
		DeletionState: deletionState,
		Provider:      s.headSync,
		Configuration: s.configuration,
		OnSpaceDelete: s.onSpaceDelete,
	}
	s.settingsObject = settings.NewSettingsObject(deps, s.id)
	s.objectSync.Init()
	s.headSync.Init(initialIds, deletionState)
	err = s.settingsObject.Init(ctx)
	if err != nil {
		return
	}
	s.cache.AddObject(s.settingsObject)
	s.syncStatus.Run()
	s.handleQueue = multiqueue.New[HandleMessage](s.handleMessage, 100)
	return nil
}

func (s *space) ObjectSync() objectsync.ObjectSync {
	return s.objectSync
}

func (s *space) HeadSync() headsync.HeadSync {
	return s.headSync
}

func (s *space) SyncStatus() syncstatus.StatusUpdater {
	return s.syncStatus
}

func (s *space) Storage() spacestorage.SpaceStorage {
	return s.storage
}

func (s *space) StoredIds() []string {
	return slice.DiscardFromSlice(s.headSync.AllIds(), func(id string) bool {
		return id == s.settingsObject.Id()
	})
}

func (s *space) DebugAllHeads() []headsync.TreeHeads {
	return s.headSync.DebugAllHeads()
}

func (s *space) DeriveTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	root, err := objecttree.DeriveObjectTreeRoot(payload, s.aclList)
	if err != nil {
		return
	}
	res = treestorage.TreeStorageCreatePayload{
		RootRawChange: root,
		Changes:       []*treechangeproto.RawTreeChangeWithId{root},
		Heads:         []string{root.Id},
	}
	return
}

func (s *space) CreateTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	root, err := objecttree.CreateObjectTreeRoot(payload, s.aclList)
	if err != nil {
		return
	}

	res = treestorage.TreeStorageCreatePayload{
		RootRawChange: root,
		Changes:       []*treechangeproto.RawTreeChangeWithId{root},
		Heads:         []string{root.Id},
	}
	return
}

func (s *space) PutTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, listener updatelistener.UpdateListener) (t objecttree.ObjectTree, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	deps := synctree.BuildDeps{
		SpaceId:        s.id,
		ObjectSync:     s.objectSync,
		Configuration:  s.configuration,
		HeadNotifiable: s.headSync,
		Listener:       listener,
		AclList:        s.aclList,
		SpaceStorage:   s.storage,
		OnClose:        s.onObjectClose,
		SyncStatus:     s.syncStatus,
		PeerGetter:     s.peerManager,
	}
	t, err = synctree.PutSyncTree(ctx, payload, deps)
	if err != nil {
		return
	}
	s.treesUsed.Add(1)
	log.Debug("incrementing counter", zap.String("id", payload.RootRawChange.Id), zap.Int32("trees", s.treesUsed.Load()), zap.String("spaceId", s.id))
	return
}

type BuildTreeOpts struct {
	Listener           updatelistener.UpdateListener
	WaitTreeRemoteSync bool
}

type HistoryTreeOpts struct {
	BeforeId string
	Include  bool
}

func (s *space) BuildTree(ctx context.Context, id string, opts BuildTreeOpts) (t objecttree.ObjectTree, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}

	deps := synctree.BuildDeps{
		SpaceId:            s.id,
		ObjectSync:         s.objectSync,
		Configuration:      s.configuration,
		HeadNotifiable:     s.headSync,
		Listener:           opts.Listener,
		AclList:            s.aclList,
		SpaceStorage:       s.storage,
		OnClose:            s.onObjectClose,
		SyncStatus:         s.syncStatus,
		WaitTreeRemoteSync: opts.WaitTreeRemoteSync,
		PeerGetter:         s.peerManager,
	}
	if t, err = synctree.BuildSyncTreeOrGetRemote(ctx, id, deps); err != nil {
		return nil, err
	}
	s.treesUsed.Add(1)
	log.Debug("incrementing counter", zap.String("id", id), zap.Int32("trees", s.treesUsed.Load()), zap.String("spaceId", s.id))
	return
}

func (s *space) BuildHistoryTree(ctx context.Context, id string, opts HistoryTreeOpts) (t objecttree.HistoryTree, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}

	params := objecttree.HistoryTreeParams{
		AclList:         s.aclList,
		BeforeId:        opts.BeforeId,
		IncludeBeforeId: opts.Include,
	}
	params.TreeStorage, err = s.storage.TreeStorage(id)
	if err != nil {
		return
	}
	return objecttree.BuildHistoryTree(params)
}

func (s *space) DeleteTree(ctx context.Context, id string) (err error) {
	return s.settingsObject.DeleteObject(id)
}

func (s *space) SpaceDeleteRawChange(ctx context.Context) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return s.settingsObject.SpaceDeleteRawChange()
}

func (s *space) DeleteSpace(ctx context.Context, deleteChange *treechangeproto.RawTreeChangeWithId) (err error) {
	return s.settingsObject.DeleteSpace(ctx, deleteChange)
}

func (s *space) HandleMessage(ctx context.Context, hm HandleMessage) (err error) {
	threadId := hm.Message.ObjectId
	hm.ReceiveTime = time.Now()
	if hm.Message.ReplyId != "" {
		threadId += hm.Message.ReplyId
		defer func() {
			_ = s.handleQueue.CloseThread(threadId)
		}()
	}
	err = s.handleQueue.Add(ctx, threadId, hm)
	if err == mb.ErrOverflowed {
		log.InfoCtx(ctx, "queue overflowed", zap.String("spaceId", s.id), zap.String("objectId", threadId))
		// skip overflowed error
		return nil
	}
	return
}

func (s *space) handleMessage(msg HandleMessage) {
	var err error
	msg.StartHandlingTime = time.Now()
	ctx := peer.CtxWithPeerId(context.Background(), msg.SenderId)
	ctx = logger.CtxWithFields(ctx, zap.Uint64("msgId", msg.Id), zap.String("senderId", msg.SenderId))
	defer func() {
		if s.metric == nil {
			return
		}
		s.metric.RequestLog(msg.PeerCtx, "space.streamOp", msg.LogFields(
			zap.Error(err),
		)...)
	}()

	if !msg.Deadline.IsZero() {
		now := time.Now()
		if now.After(msg.Deadline) {
			log.InfoCtx(ctx, "skip message: deadline exceed")
			err = context.DeadlineExceeded
			return
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, msg.Deadline)
		defer cancel()
	}

	if err = s.objectSync.HandleMessage(ctx, msg.SenderId, msg.Message); err != nil {
		if msg.Message.ObjectId != "" {
			// cleanup thread on error
			_ = s.handleQueue.CloseThread(msg.Message.ObjectId)
		}
		log.InfoCtx(ctx, "handleMessage error", zap.Error(err))
	}
}

func (s *space) onObjectClose(id string) {
	s.treesUsed.Add(-1)
	log.Debug("decrementing counter", zap.String("id", id), zap.Int32("trees", s.treesUsed.Load()), zap.String("spaceId", s.id))
	_ = s.handleQueue.CloseThread(id)
}

func (s *space) onSpaceDelete() {
	err := s.storage.SetSpaceDeleted()
	if err != nil {
		log.Debug("failed to set space deleted")
	}
	s.isDeleted.Swap(true)
}

func (s *space) Close() error {
	if s.isClosed.Swap(true) {
		log.Warn("call space.Close on closed space", zap.String("id", s.id))
		return nil
	}
	log.With(zap.String("id", s.id)).Debug("space is closing")

	var mError errs.Group
	if err := s.handleQueue.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.headSync.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.objectSync.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.settingsObject.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.aclList.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.storage.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.syncStatus.Close(); err != nil {
		mError.Add(err)
	}
	log.With(zap.String("id", s.id)).Debug("space closed")
	return mError.Err()
}

func (s *space) TryClose(objectTTL time.Duration) (close bool, err error) {
	if time.Now().Sub(s.objectSync.LastUsage()) < objectTTL {
		return false, nil
	}
	locked := s.treesUsed.Load() > 1
	log.With(zap.Int32("trees used", s.treesUsed.Load()), zap.Bool("locked", locked), zap.String("spaceId", s.id)).Debug("space lock status check")
	if locked {
		return false, nil
	}
	return true, s.Close()
}
