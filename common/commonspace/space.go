package commonspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncacl"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	aclstorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	tree "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
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

const SpaceTypeDerived = "derived.space"

type SpaceDerivePayload struct {
	SigningKey    signingkey.PrivKey
	EncryptionKey encryptionkey.PrivKey
}

func NewSpaceId(id string, repKey uint64) string {
	return fmt.Sprintf("%s.%d", id, repKey)
}

type Space interface {
	Id() string
	StoredIds() []string

	SpaceSyncRpc() RpcHandler

	DeriveTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener updatelistener.UpdateListener) (tree.ObjectTree, error)
	CreateTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener updatelistener.UpdateListener) (tree.ObjectTree, error)
	BuildTree(ctx context.Context, id string, listener updatelistener.UpdateListener) (tree.ObjectTree, error)

	Close() error
}

type space struct {
	id string
	mu sync.RWMutex

	rpc *rpcHandler

	syncService   syncservice.SyncService
	diffService   diffservice.DiffService
	storage       storage.SpaceStorage
	cache         treegetter.TreeGetter
	account       account.Service
	aclList       *syncacl.SyncACL
	configuration nodeconf.Configuration

	isClosed atomic.Bool
}

func (s *space) LastUsage() time.Time {
	return s.syncService.LastUsage()
}

func (s *space) Id() string {
	return s.id
}

func (s *space) Init(ctx context.Context) (err error) {
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
	objectGetter := newCommonSpaceGetter(s.id, s.aclList, s.cache)
	s.syncService.Init(objectGetter)
	s.diffService.Init(initialIds)
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
		CreateStorage:  s.storage.CreateTreeStorage,
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
		CreateStorage:  s.storage.CreateTreeStorage,
	}
	return synctree.CreateSyncTree(ctx, deps)
}

func (s *space) BuildTree(ctx context.Context, id string, listener updatelistener.UpdateListener) (t tree.ObjectTree, err error) {
	if s.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	getTreeRemote := func() (*spacesyncproto.ObjectSyncMessage, error) {
		// TODO: add empty context handling (when this is not happening due to head update)
		peerId, err := syncservice.GetPeerIdFromStreamContext(ctx)
		if err != nil {
			return nil, err
		}
		return s.syncService.StreamPool().SendSync(
			peerId,
			synctree.GetRequestFactory().CreateNewTreeRequest(id),
		)
	}

	store, err := s.storage.TreeStorage(id)
	if err != nil && err != aclstorage.ErrUnknownTreeId {
		return
	}

	isFirstBuild := false
	if err == aclstorage.ErrUnknownTreeId {
		isFirstBuild = true
		var resp *spacesyncproto.ObjectSyncMessage
		resp, err = getTreeRemote()
		if err != nil {
			return
		}
		fullSyncResp := resp.GetContent().GetFullSyncResponse()

		payload := aclstorage.TreeStorageCreatePayload{
			TreeId:        resp.TreeId,
			RootRawChange: resp.RootChange,
			Changes:       fullSyncResp.Changes,
			Heads:         fullSyncResp.Heads,
		}

		// basically building tree with inmemory storage and validating that it was without errors
		err = tree.ValidateRawTree(payload, s.aclList)
		if err != nil {
			return
		}
		// now we are sure that we can save it to the storage
		store, err = s.storage.CreateTreeStorage(payload)
		if err != nil {
			return
		}
	}
	deps := synctree.BuildDeps{
		SpaceId:        s.id,
		StreamPool:     s.syncService.StreamPool(),
		Configuration:  s.configuration,
		HeadNotifiable: s.diffService,
		Listener:       listener,
		AclList:        s.aclList,
		Storage:        store,
	}
	return synctree.BuildSyncTree(ctx, isFirstBuild, deps)
}

func (s *space) Close() error {
	log.With(zap.String("id", s.id)).Debug("space is closing")
	defer func() {
		s.isClosed.Store(true)
		log.With(zap.String("id", s.id)).Debug("space closed")
	}()
	s.diffService.Close()
	s.syncService.Close()
	return s.storage.Close()
}
