package commonspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncacl"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"
)

var ErrSpaceClosed = errors.New("space is closed")

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

const (
	SpaceTypeDerived          = "derived.space"
	SettingsSyncPeriodSeconds = 10
)

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
	return fmt.Sprintf("%s.%d", id, repKey)
}

type Space interface {
	Id() string
	Init(ctx context.Context) error

	StoredIds() []string
	Description() (SpaceDescription, error)

	SpaceSyncRpc() RpcHandler

	DeriveTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener updatelistener.UpdateListener) (tree.ObjectTree, error)
	CreateTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener updatelistener.UpdateListener) (tree.ObjectTree, error)
	BuildTree(ctx context.Context, id string, listener updatelistener.UpdateListener) (tree.ObjectTree, error)
	DeleteTree(ctx context.Context, id string) (err error)

	Close() error
}

type space struct {
	id     string
	mu     sync.RWMutex
	header *spacesyncproto.RawSpaceHeaderWithId

	rpc *rpcHandler

	syncService      syncservice.SyncService
	diffService      diffservice.DiffService
	storage          storage.SpaceStorage
	cache            treegetter.TreeGetter
	account          account.Service
	aclList          *syncacl.SyncACL
	configuration    nodeconf.Configuration
	settingsDocument settingsdocument.SettingsDocument

	isClosed atomic.Bool
}

func (s *space) LastUsage() time.Time {
	return s.syncService.LastUsage()
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
	aclStorage, err := s.storage.ACLStorage()
	if err != nil {
		return
	}
	aclList, err := list.BuildACLListWithIdentity(s.account.Account(), aclStorage)
	if err != nil {
		return
	}
	s.aclList = syncacl.NewSyncACL(aclList, s.syncService.StreamPool())

	deletionState := deletionstate.NewDeletionState(s.storage)
	deps := settingsdocument.Deps{
		BuildFunc: func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error) {
			res, err := s.BuildTree(ctx, id, listener)
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
	s.settingsDocument, err = settingsdocument.NewSettingsDocument(deps, s.id)
	if err != nil {
		return
	}

	objectGetter := newCommonSpaceGetter(s.id, s.aclList, s.cache, s.settingsDocument)
	s.syncService.Init(objectGetter)
	s.diffService.Init(initialIds, deletionState)
	err = s.settingsDocument.Init(ctx)
	if err != nil {
		return
	}

	return nil
}

func (s *space) SpaceSyncRpc() RpcHandler {
	return s.rpc
}

func (s *space) SyncService() syncservice.SyncService {
	return s.syncService
}

func (s *space) DiffService() diffservice.DiffService {
	return s.diffService
}

func (s *space) StoredIds() []string {
	return s.diffService.AllIds()
}

func (s *space) DeriveTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener updatelistener.UpdateListener) (tr tree.ObjectTree, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	deps := synctree.CreateDeps{
		SpaceId:        s.id,
		Payload:        payload,
		StreamPool:     s.syncService.StreamPool(),
		Configuration:  s.configuration,
		HeadNotifiable: s.diffService,
		Listener:       listener,
		AclList:        s.aclList,
		SpaceStorage:   s.storage,
	}
	return synctree.DeriveSyncTree(ctx, deps)
}

func (s *space) CreateTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener updatelistener.UpdateListener) (tr tree.ObjectTree, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	deps := synctree.CreateDeps{
		SpaceId:        s.id,
		Payload:        payload,
		StreamPool:     s.syncService.StreamPool(),
		Configuration:  s.configuration,
		HeadNotifiable: s.diffService,
		Listener:       listener,
		AclList:        s.aclList,
		SpaceStorage:   s.storage,
	}
	return synctree.CreateSyncTree(ctx, deps)
}

func (s *space) BuildTree(ctx context.Context, id string, listener updatelistener.UpdateListener) (t tree.ObjectTree, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	deps := synctree.BuildDeps{
		SpaceId:        s.id,
		StreamPool:     s.syncService.StreamPool(),
		Configuration:  s.configuration,
		HeadNotifiable: s.diffService,
		Listener:       listener,
		AclList:        s.aclList,
		SpaceStorage:   s.storage,
	}
	return synctree.BuildSyncTreeOrGetRemote(ctx, id, deps)
}

func (s *space) DeleteTree(ctx context.Context, id string) (err error) {
	return s.settingsDocument.DeleteObject(id)
}

func (s *space) Close() error {
	log.With(zap.String("id", s.id)).Debug("space is closing")
	defer func() {
		s.isClosed.Store(true)
		log.With(zap.String("id", s.id)).Debug("space closed")
	}()
	var mError errs.Group
	if err := s.diffService.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.syncService.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.settingsDocument.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.aclList.Close(); err != nil {
		mError.Add(err)
	}
	if err := s.storage.Close(); err != nil {
		mError.Add(err)
	}
	return mError.Err()
}
