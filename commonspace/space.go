package commonspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app/ocache"
	"github.com/anytypeio/any-sync/commonspace/headsync"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/acl/syncacl"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/settings"
	"github.com/anytypeio/any-sync/commonspace/settings/deletionstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSpaceClosed       = errors.New("space is closed")
	ErrPutNotImplemented = errors.New("put tree is not implemented")
)

type SpaceCreatePayload struct {
	// SigningKey is the signing key of the owner
	SigningKey signingkey.PrivKey
	// EncryptionKey is the encryption key of the owner
	EncryptionKey encryptionkey.PrivKey
	// SpaceType is an arbitrary string
	SpaceType string
	// ReadKey is a first symmetric encryption key for a space
	ReadKey []byte
	// ReplicationKey is a key which is to be used to determine the node where the space should be held
	ReplicationKey uint64
}

const SpaceTypeDerived = "derived.space"

type SpaceDerivePayload struct {
	SigningKey    signingkey.PrivKey
	EncryptionKey encryptionkey.PrivKey
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
	ocache.ObjectLocker
	ocache.ObjectLastUsage

	Id() string
	Init(ctx context.Context) error

	StoredIds() []string
	DebugAllHeads() []headsync.TreeHeads
	Description() (SpaceDescription, error)

	SpaceSyncRpc() RpcHandler

	DeriveTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error)
	CreateTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error)
	PutTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, listener updatelistener.UpdateListener) (t objecttree.ObjectTree, err error)
	BuildTree(ctx context.Context, id string, opts BuildTreeOpts) (t objecttree.ObjectTree, err error)
	DeleteTree(ctx context.Context, id string) (err error)

	SyncStatus() syncstatus.StatusUpdater
	Storage() spacestorage.SpaceStorage

	Close() error
}

type space struct {
	id     string
	mu     sync.RWMutex
	header *spacesyncproto.RawSpaceHeaderWithId

	rpc *rpcHandler

	objectSync     objectsync.ObjectSync
	headSync       headsync.HeadSync
	syncStatus     syncstatus.StatusUpdater
	storage        spacestorage.SpaceStorage
	cache          treegetter.TreeGetter
	account        accountservice.Service
	aclList        *syncacl.SyncAcl
	configuration  nodeconf.Configuration
	settingsObject settings.SettingsObject

	isClosed  atomic.Bool
	treesUsed atomic.Int32
}

func (s *space) LastUsage() time.Time {
	return s.objectSync.LastUsage()
}

func (s *space) Locked() bool {
	locked := s.treesUsed.Load() > 1
	log.With(zap.Int32("trees used", s.treesUsed.Load()), zap.Bool("locked", locked)).Debug("space lock status check")
	return locked
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
	s.rpc = &rpcHandler{s: s}
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
	s.aclList = syncacl.NewSyncAcl(aclList, s.objectSync.StreamPool())

	deletionState := deletionstate.NewDeletionState(s.storage)
	deps := settings.Deps{
		BuildFunc: func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error) {
			res, err := s.BuildTree(ctx, id, BuildTreeOpts{
				Listener:           listener,
				WaitTreeRemoteSync: false,
			})
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
	}
	s.settingsObject = settings.NewSettingsObject(deps, s.id)

	objectGetter := newCommonSpaceGetter(s.id, s.aclList, s.cache, s.settingsObject)
	s.objectSync.Init(objectGetter)
	s.headSync.Init(initialIds, deletionState)
	err = s.settingsObject.Init(ctx)
	if err != nil {
		return
	}
	s.syncStatus.Run()

	return nil
}

func (s *space) SpaceSyncRpc() RpcHandler {
	return s.rpc
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
	return s.headSync.AllIds()
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
		TreeUsage:      &s.treesUsed,
		SyncStatus:     s.syncStatus,
	}
	return synctree.PutSyncTree(ctx, payload, deps)
}

type BuildTreeOpts struct {
	Listener           updatelistener.UpdateListener
	WaitTreeRemoteSync bool
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
		TreeUsage:          &s.treesUsed,
		SyncStatus:         s.syncStatus,
		WaitTreeRemoteSync: opts.WaitTreeRemoteSync,
	}
	return synctree.BuildSyncTreeOrGetRemote(ctx, id, deps)
}

func (s *space) DeleteTree(ctx context.Context, id string) (err error) {
	return s.settingsObject.DeleteObject(id)
}

func (s *space) Close() error {
	log.With(zap.String("id", s.id)).Debug("space is closing")
	defer func() {
		s.isClosed.Store(true)
		log.With(zap.String("id", s.id)).Debug("space closed")
	}()
	var mError errs.Group
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

	return mError.Err()
}
