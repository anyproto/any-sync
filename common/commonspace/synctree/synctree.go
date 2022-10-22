package synctree

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
)

var ErrSyncTreeClosed = errors.New("sync tree is closed")

// SyncTree sends head updates to sync service and also sends new changes to update listener
type SyncTree struct {
	tree.ObjectTree
	synchandler.SyncHandler
	syncClient syncservice.SyncClient
	listener   updatelistener.UpdateListener
	isClosed   bool
}

var log = logger.NewNamed("commonspace.synctree").Sugar()

var createDerivedObjectTree = tree.CreateDerivedObjectTree
var createObjectTree = tree.CreateObjectTree
var buildObjectTree = tree.BuildObjectTree

type SyncTreeCreateDeps struct {
	Payload       tree.ObjectTreeCreatePayload
	SyncClient    syncservice.SyncClient
	Listener      updatelistener.UpdateListener
	AclList       list.ACLList
	CreateStorage storage.TreeStorageCreatorFunc
}

type SyncTreeBuildDeps struct {
	SyncClient syncservice.SyncClient
	Listener   updatelistener.UpdateListener
	AclList    list.ACLList
	Storage    storage.TreeStorage
}

func DeriveSyncTree(
	ctx context.Context,
	deps SyncTreeCreateDeps) (t tree.ObjectTree, err error) {
	t, err = createDerivedObjectTree(deps.Payload, deps.AclList, deps.CreateStorage)
	if err != nil {
		return
	}
	syncTree := &SyncTree{
		ObjectTree: t,
		syncClient: deps.SyncClient,
		listener:   deps.Listener,
	}
	syncHandler := newSyncTreeHandler(syncTree, deps.SyncClient)
	syncTree.SyncHandler = syncHandler
	t = syncTree

	headUpdate := deps.SyncClient.CreateHeadUpdate(t, nil)
	err = deps.SyncClient.BroadcastAsync(headUpdate)
	return
}

func CreateSyncTree(
	ctx context.Context,
	deps SyncTreeCreateDeps) (t tree.ObjectTree, err error) {
	t, err = createObjectTree(deps.Payload, deps.AclList, deps.CreateStorage)
	if err != nil {
		return
	}
	syncTree := &SyncTree{
		ObjectTree: t,
		syncClient: deps.SyncClient,
		listener:   deps.Listener,
	}
	syncHandler := newSyncTreeHandler(syncTree, deps.SyncClient)
	syncTree.SyncHandler = syncHandler
	t = syncTree

	headUpdate := deps.SyncClient.CreateHeadUpdate(t, nil)
	err = deps.SyncClient.BroadcastAsync(headUpdate)
	return
}

func BuildSyncTree(
	ctx context.Context,
	isFirstBuild bool,
	deps SyncTreeBuildDeps) (t tree.ObjectTree, err error) {

	t, err = buildObjectTree(deps.Storage, deps.AclList)
	if err != nil {
		return
	}
	syncTree := &SyncTree{
		ObjectTree: t,
		syncClient: deps.SyncClient,
		listener:   deps.Listener,
	}
	syncHandler := newSyncTreeHandler(syncTree, deps.SyncClient)
	syncTree.SyncHandler = syncHandler
	t = syncTree

	headUpdate := syncTree.syncClient.CreateHeadUpdate(t, nil)
	// here we will have different behaviour based on who is sending this update
	if isFirstBuild {
		// send to everybody, because everybody should know that the node or client got new tree
		err = syncTree.syncClient.BroadcastAsync(headUpdate)
	} else {
		// send either to everybody if client or to replica set if node
		err = syncTree.syncClient.BroadcastAsyncOrSendResponsible(headUpdate)
	}
	return
}

func (s *SyncTree) AddContent(ctx context.Context, content tree.SignableChangeContent) (res tree.AddResult, err error) {
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
	}
	res, err = s.ObjectTree.AddContent(ctx, content)
	if err != nil {
		return
	}
	headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
	err = s.syncClient.BroadcastAsync(headUpdate)
	return
}

func (s *SyncTree) AddRawChanges(ctx context.Context, changes ...*treechangeproto.RawTreeChangeWithId) (res tree.AddResult, err error) {
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
	}
	res, err = s.ObjectTree.AddRawChanges(ctx, changes...)
	if err != nil {
		return
	}
	if s.listener != nil {
		switch res.Mode {
		case tree.Nothing:
			return
		case tree.Append:
			s.listener.Update(s)
		case tree.Rebuild:
			s.listener.Rebuild(s)
		}
	}
	if res.Mode != tree.Nothing {
		headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
		err = s.syncClient.BroadcastAsync(headUpdate)
	}
	return
}

func (s *SyncTree) Close() (err error) {
	log.With("id", s.ID()).Debug("closing sync tree")
	s.Lock()
	defer s.Unlock()
	log.With("id", s.ID()).Debug("taken lock on sync tree")
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
	}
	s.isClosed = true
	return
}

func (s *SyncTree) Tree() tree.ObjectTree {
	return s
}
