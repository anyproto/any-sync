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
	objTree     tree.ObjectTree
	syncService syncservice.SyncService
}

func CreateSyncTree(
	syncService syncservice.SyncService,
	payload tree.ObjectTreeCreatePayload,
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

func (s *SyncTree) Lock() {
	s.objTree.Lock()
}

func (s *SyncTree) Unlock() {
	s.objTree.Unlock()
}

func (s *SyncTree) RLock() {
	s.objTree.RLock()
}

func (s *SyncTree) RUnlock() {
	s.objTree.RUnlock()
}

func (s *SyncTree) ID() string {
	return s.objTree.ID()
}

func (s *SyncTree) Header() *aclpb.TreeHeader {
	return s.objTree.Header()
}

func (s *SyncTree) Heads() []string {
	return s.objTree.Heads()
}

func (s *SyncTree) Root() *tree.Change {
	return s.objTree.Root()
}

func (s *SyncTree) HasChange(id string) bool {
	return s.objTree.HasChange(id)
}

func (s *SyncTree) Iterate(convert tree.ChangeConvertFunc, iterate tree.ChangeIterateFunc) error {
	return s.objTree.Iterate(convert, iterate)
}

func (s *SyncTree) IterateFrom(id string, convert tree.ChangeConvertFunc, iterate tree.ChangeIterateFunc) error {
	return s.objTree.IterateFrom(id, convert, iterate)
}

func (s *SyncTree) SnapshotPath() []string {
	return s.objTree.SnapshotPath()
}

func (s *SyncTree) ChangesAfterCommonSnapshot(snapshotPath, heads []string) ([]*aclpb.RawTreeChangeWithId, error) {
	return s.objTree.ChangesAfterCommonSnapshot(snapshotPath, heads)
}

func (s *SyncTree) Storage() storage.TreeStorage {
	return s.objTree.Storage()
}

func (s *SyncTree) DebugDump() (string, error) {
	return s.objTree.DebugDump()
}

func (s *SyncTree) AddContent(ctx context.Context, content tree.SignableChangeContent) (res tree.AddResult, err error) {
	res, err = s.objTree.AddContent(ctx, content)
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
	res, err = s.objTree.AddRawChanges(ctx, changes...)
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

func (s *SyncTree) Close() error {
	return s.objTree.Close()
}
