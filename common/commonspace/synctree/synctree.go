package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
)

type SyncTree struct {
	tree.ObjectTree
	syncService syncservice.SyncService
}

func CreateSyncTree(
	payload tree.ObjectTreeCreatePayload,
	syncService syncservice.SyncService,
	listener tree.ObjectTreeUpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (t tree.ObjectTree, err error) {
	t, err = tree.CreateObjectTree(payload, listener, aclList, createStorage)
	if err != nil {
		return
	}

	// TODO: use context where it is needed
	err = syncService.NotifyHeadUpdate(context.Background(), t.ID(), t.Header(), &spacesyncproto.ObjectHeadUpdate{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	})
	return
}

func BuildSyncTree(
	syncService syncservice.SyncService,
	treeStorage storage.TreeStorage,
	listener tree.ObjectTreeUpdateListener,
	aclList list.ACLList) (t tree.ObjectTree, err error) {
	return buildSyncTree(syncService, treeStorage, listener, aclList)
}

func buildSyncTree(
	syncService syncservice.SyncService,
	treeStorage storage.TreeStorage,
	listener tree.ObjectTreeUpdateListener,
	aclList list.ACLList) (t tree.ObjectTree, err error) {
	t, err = tree.BuildObjectTree(treeStorage, listener, aclList)
	if err != nil {
		return
	}

	// TODO: use context where it is needed
	err = syncService.NotifyHeadUpdate(context.Background(), t.ID(), t.Header(), &spacesyncproto.ObjectHeadUpdate{
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

func (s *SyncTree) AddRawChanges(ctx context.Context, changes ...*aclpb.RawTreeChangeWithId) (res tree.AddResult, err error) {
	res, err = s.AddRawChanges(ctx, changes...)
	if err != nil || res.Mode == tree.Nothing {
		return
	}
	err = s.syncService.NotifyHeadUpdate(ctx, s.ID(), s.Header(), &spacesyncproto.ObjectHeadUpdate{
		Heads:        res.Heads,
		Changes:      res.Added,
		SnapshotPath: s.SnapshotPath(),
	})
	return
}
