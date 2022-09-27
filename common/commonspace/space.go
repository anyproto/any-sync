package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	treestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"sync"
)

type Space interface {
	Id() string

	SpaceSyncRpc() RpcHandler
	SyncService() syncservice.SyncService
	DiffService() diffservice.DiffService

	CreateTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener synctree.UpdateListener) (tree.ObjectTree, error)
	BuildTree(ctx context.Context, id string, listener synctree.UpdateListener) (tree.ObjectTree, error)

	Close() error
}

type space struct {
	id string
	mu sync.RWMutex

	rpc *rpcHandler

	syncService syncservice.SyncService
	diffService diffservice.DiffService
	storage     storage.Storage
	cache       cache.TreeCache
	aclList     list.ACLList
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
	s.diffService.Init(initialIds)
	s.syncService.Init()
	// basically this provides access for the external cache to use space's tree building functions
	s.cache.SetBuildFunc(s.BuildTree)
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

func (s *space) CreateTree(ctx context.Context, payload tree.ObjectTreeCreatePayload, listener synctree.UpdateListener) (tree.ObjectTree, error) {
	return synctree.CreateSyncTree(ctx, payload, s.syncService, listener, s.aclList, s.storage.CreateTreeStorage)
}

func (s *space) BuildTree(ctx context.Context, id string, listener synctree.UpdateListener) (t tree.ObjectTree, err error) {
	getTreeRemote := func() (*spacesyncproto.ObjectSyncMessage, error) {
		// TODO: add empty context handling (when this is not happening due to head update)
		peerId, err := syncservice.GetPeerIdFromStreamContext(ctx)
		if err != nil {
			return nil, err
		}

		return s.syncService.StreamPool().SendSync(
			peerId,
			spacesyncproto.WrapFullRequest(&spacesyncproto.ObjectFullSyncRequest{}, nil, id, ""),
		)
	}

	store, err := s.storage.Storage(id)
	if err != nil && err != treestorage.ErrUnknownTreeId {
		return
	}

	if err == treestorage.ErrUnknownTreeId {
		var resp *spacesyncproto.ObjectSyncMessage
		resp, err = getTreeRemote()
		if err != nil {
			return
		}
		fullSyncResp := resp.GetContent().GetFullSyncResponse()

		payload := treestorage.TreeStorageCreatePayload{
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
	return synctree.BuildSyncTree(ctx, s.syncService, store.(treestorage.TreeStorage), listener, s.aclList)
}

func (s *space) Close() error {
	s.diffService.Close()
	s.cache.Close()
	return s.syncService.Close()
}
