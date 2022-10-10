package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
)

// SyncTree sends head updates to sync service and also sends new changes to update listener
type SyncTree struct {
	tree.ObjectTree
	syncClient syncservice.SyncClient
	listener   updatelistener.UpdateListener
}

var createDerivedObjectTree = tree.CreateDerivedObjectTree
var createObjectTree = tree.CreateObjectTree
var buildObjectTree = tree.BuildObjectTree

func DeriveSyncTree(
	ctx context.Context,
	payload tree.ObjectTreeCreatePayload,
	syncClient syncservice.SyncClient,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (t tree.ObjectTree, err error) {
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
	payload tree.ObjectTreeCreatePayload,
	syncClient syncservice.SyncClient,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (t tree.ObjectTree, err error) {
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
	aclList list.ACLList) (t tree.ObjectTree, err error) {
	return buildSyncTree(ctx, syncClient, treeStorage, listener, aclList)
}

func buildSyncTree(
	ctx context.Context,
	syncClient syncservice.SyncClient,
	treeStorage storage.TreeStorage,
	listener updatelistener.UpdateListener,
	aclList list.ACLList) (t tree.ObjectTree, err error) {
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
	err = syncClient.BroadcastAsyncOrSendResponsible(headUpdate)
	return
}

func (s *SyncTree) AddContent(ctx context.Context, content tree.SignableChangeContent) (res tree.AddResult, err error) {
	res, err = s.ObjectTree.AddContent(ctx, content)
	if err != nil {
		return
	}
	headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
	err = s.syncClient.BroadcastAsync(headUpdate)
	return
}

func (s *SyncTree) AddRawChanges(ctx context.Context, changes ...*treechangeproto.RawTreeChangeWithId) (res tree.AddResult, err error) {
	res, err = s.ObjectTree.AddRawChanges(ctx, changes...)
	if err != nil {
		return
	}
	switch res.Mode {
	case tree.Nothing:
		return
	case tree.Append:
		s.listener.Update(s)
	case tree.Rebuild:
		s.listener.Rebuild(s)
	}

	headUpdate := s.syncClient.CreateHeadUpdate(s, res.Added)
	err = s.syncClient.BroadcastAsync(headUpdate)
	return
}

func (s *SyncTree) Tree() tree.ObjectTree {
	return s
}
