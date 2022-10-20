package synctree

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	tree2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
)

var ErrSyncTreeClosed = errors.New("sync tree is closed")

// SyncTree sends head updates to sync service and also sends new changes to update listener
type SyncTree struct {
	tree2.ObjectTree
	syncClient syncservice.SyncClient
	listener   updatelistener.UpdateListener
	isClosed   bool
}

var log = logger.NewNamed("commonspace.synctree").Sugar()

var createDerivedObjectTree = tree2.CreateDerivedObjectTree
var createObjectTree = tree2.CreateObjectTree
var buildObjectTree = tree2.BuildObjectTree

func DeriveSyncTree(
	ctx context.Context,
	payload tree2.ObjectTreeCreatePayload,
	syncClient syncservice.SyncClient,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (t tree2.ObjectTree, err error) {
	t, err = createDerivedObjectTree(payload, aclList, createStorage)
	if err != nil {
		return
	}
	t = &SyncTree{
		ObjectTree: t,
		syncClient: syncClient,
		listener:   listener,
	}

	headUpdate := syncClient.CreateHeadUpdate(t, nil)
	err = syncClient.BroadcastAsync(headUpdate)
	return
}

func CreateSyncTree(
	ctx context.Context,
	payload tree2.ObjectTreeCreatePayload,
	syncClient syncservice.SyncClient,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (t tree2.ObjectTree, err error) {
	t, err = createObjectTree(payload, aclList, createStorage)
	if err != nil {
		return
	}
	t = &SyncTree{
		ObjectTree: t,
		syncClient: syncClient,
		listener:   listener,
	}

	headUpdate := syncClient.CreateHeadUpdate(t, nil)
	err = syncClient.BroadcastAsync(headUpdate)
	return
}

func BuildSyncTree(
	ctx context.Context,
	syncClient syncservice.SyncClient,
	treeStorage storage.TreeStorage,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	isFirstBuild bool) (t tree2.ObjectTree, err error) {
	return buildSyncTree(ctx, syncClient, treeStorage, listener, aclList, isFirstBuild)
}

func buildSyncTree(
	ctx context.Context,
	syncClient syncservice.SyncClient,
	treeStorage storage.TreeStorage,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	isFirstBuild bool) (t tree2.ObjectTree, err error) {
	t, err = buildObjectTree(treeStorage, aclList)
	if err != nil {
		return
	}
	t = &SyncTree{
		ObjectTree: t,
		syncClient: syncClient,
		listener:   listener,
	}

	headUpdate := syncClient.CreateHeadUpdate(t, nil)
	// here we will have different behaviour based on who is sending this update
	if isFirstBuild {
		// send to everybody, because everybody should know that the node or client got new tree
		err = syncClient.BroadcastAsync(headUpdate)
	} else {
		// send either to everybody if client or to replica set if node
		err = syncClient.BroadcastAsyncOrSendResponsible(headUpdate)
	}
	return
}

func (s *SyncTree) AddContent(ctx context.Context, content tree2.SignableChangeContent) (res tree2.AddResult, err error) {
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

func (s *SyncTree) AddRawChanges(ctx context.Context, changes ...*treechangeproto.RawTreeChangeWithId) (res tree2.AddResult, err error) {
	if s.isClosed {
		err = ErrSyncTreeClosed
		return
	}
	res, err = s.ObjectTree.AddRawChanges(ctx, changes...)
	if err != nil {
		return
	}
	switch res.Mode {
	case tree2.Nothing:
		return
	case tree2.Append:
		s.listener.Update(s)
	case tree2.Rebuild:
		s.listener.Rebuild(s)
	}

	headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
	err = s.syncClient.BroadcastAsync(headUpdate)
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

func (s *SyncTree) Tree() tree2.ObjectTree {
	return s
}
