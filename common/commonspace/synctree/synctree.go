package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
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
	syncService syncservice.SyncService
	listener    updatelistener.UpdateListener
}

func DeriveSyncTree(
	ctx context.Context,
	payload tree.ObjectTreeCreatePayload,
	syncService syncservice.SyncService,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (t tree.ObjectTree, err error) {
	t, err = tree.CreateDerivedObjectTree(payload, aclList, createStorage)
	if err != nil {
		return
	}
	t = &SyncTree{
		ObjectTree:  t,
		syncService: syncService,
		listener:    listener,
	}

	err = syncService.NotifyHeadUpdate(ctx, t.ID(), t.Header(), &spacesyncproto.ObjectHeadUpdate{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	})
	return
}

func CreateSyncTree(
	ctx context.Context,
	payload tree.ObjectTreeCreatePayload,
	syncService syncservice.SyncService,
	listener updatelistener.UpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (t tree.ObjectTree, err error) {
	t, err = tree.CreateObjectTree(payload, aclList, createStorage)
	if err != nil {
		return
	}
	t = &SyncTree{
		ObjectTree:  t,
		syncService: syncService,
		listener:    listener,
	}

	err = syncService.NotifyHeadUpdate(ctx, t.ID(), t.Header(), &spacesyncproto.ObjectHeadUpdate{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	})
	return
}

func BuildSyncTree(
	ctx context.Context,
	syncService syncservice.SyncService,
	treeStorage storage.TreeStorage,
	listener updatelistener.UpdateListener,
	aclList list.ACLList) (t tree.ObjectTree, err error) {
	return buildSyncTree(ctx, syncService, treeStorage, listener, aclList)
}

func buildSyncTree(
	ctx context.Context,
	syncService syncservice.SyncService,
	treeStorage storage.TreeStorage,
	listener updatelistener.UpdateListener,
	aclList list.ACLList) (t tree.ObjectTree, err error) {
	t, err = tree.BuildObjectTree(treeStorage, aclList)
	if err != nil {
		return
	}
	t = &SyncTree{
		ObjectTree:  t,
		syncService: syncService,
		listener:    listener,
	}

	err = syncService.NotifyHeadUpdate(ctx, t.ID(), t.Header(), &spacesyncproto.ObjectHeadUpdate{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	})
	return
}

func (s *SyncTree) AddContent(ctx context.Context, content tree.SignableChangeContent) (res tree.AddResult, err error) {
	res, err = s.AddContent(ctx, content)
	if err != nil {
		return
	}
	err = s.syncService.NotifyHeadUpdate(ctx, s.ID(), s.Header(), &spacesyncproto.ObjectHeadUpdate{
		Heads:        res.Heads,
		Changes:      res.Added,
		SnapshotPath: s.SnapshotPath(),
	})
	return
}

func (s *SyncTree) AddRawChanges(ctx context.Context, changes ...*treechangeproto.RawTreeChangeWithId) (res tree.AddResult, err error) {
	res, err = s.AddRawChanges(ctx, changes...)
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

	err = s.syncService.NotifyHeadUpdate(ctx, s.ID(), s.Header(), &spacesyncproto.ObjectHeadUpdate{
		Heads:        res.Heads,
		Changes:      res.Added,
		SnapshotPath: s.SnapshotPath(),
	})
	return
}

func (s *SyncTree) Tree() tree.ObjectTree {
	return s
}
