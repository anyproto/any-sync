package deletionstate

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage/mock_headstorage"
)

type fixture struct {
	ctrl        *gomock.Controller
	delState    *objectDeletionState
	headStorage *mock_headstorage.MockHeadStorage
}

func newFixture(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	headStorage := mock_headstorage.NewMockHeadStorage(ctrl)
	delState := New().(*objectDeletionState)
	delState.storage = headStorage
	return &fixture{
		ctrl:        ctrl,
		delState:    delState,
		headStorage: headStorage,
	}
}

func (fx *fixture) stop() {
	fx.ctrl.Finish()
}

func TestDeletionState_Add(t *testing.T) {
	t.Run("add new", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		id := "newId"
		var queued []string
		fx.delState.AddObserver(func(ids []string) {
			queued = ids
		})
		fx.headStorage.EXPECT().UpdateEntry(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, update headstorage.HeadsUpdate) error {
			require.Equal(t, headstorage.DeletedStatusQueued, *update.DeletedStatus)
			return nil
		})
		fx.delState.Add(map[string]struct{}{id: {}})
		require.Contains(t, fx.delState.queued, id)
		require.Equal(t, []string{id}, queued)
	})

	t.Run("add existing queued", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		id := "newId"
		fx.delState.queued[id] = struct{}{}
		fx.delState.Add(map[string]struct{}{id: {}})
	})

	t.Run("add existing deleted", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		id := "newId"
		fx.delState.deleted[id] = struct{}{}
		fx.delState.Add(map[string]struct{}{id: {}})
	})
}

func TestDeletionState_GetQueued(t *testing.T) {
	fx := newFixture(t)
	defer fx.stop()

	fx.delState.queued["id1"] = struct{}{}
	fx.delState.queued["id2"] = struct{}{}

	queued := fx.delState.GetQueued()
	sort.Strings(queued)
	require.Equal(t, []string{"id1", "id2"}, queued)
}

func TestDeletionState_FilterJoin(t *testing.T) {
	fx := newFixture(t)
	defer fx.stop()

	fx.delState.queued["id1"] = struct{}{}
	fx.delState.queued["id2"] = struct{}{}

	filtered := fx.delState.Filter([]string{"id3", "id2"})
	require.Equal(t, []string{"id3"}, filtered)
}

func TestDeletionState_Delete(t *testing.T) {
	fx := newFixture(t)
	defer fx.stop()

	id := "deletedId"
	fx.delState.queued[id] = struct{}{}
	fx.headStorage.EXPECT().UpdateEntry(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, update headstorage.HeadsUpdate) error {
		require.Equal(t, headstorage.DeletedStatusDeleted, *update.DeletedStatus)
		return nil
	})
	err := fx.delState.Delete(id)
	require.NoError(t, err)
	require.Contains(t, fx.delState.deleted, id)
	require.NotContains(t, fx.delState.queued, id)
}

func TestDeletionState_Run_OrphanedChildren(t *testing.T) {
	for _, scan := range []bool{false, true} {
		name := "query per deleted id"
		if scan {
			name = "scan"
		}
		t.Run(name, func(t *testing.T) {
			testRunOrphanedChildren(t, scan)
		})
	}
}

// setOrphanScan makes Run use the scan for any number of deleted ids, or never
func setOrphanScan(t *testing.T, scan bool) {
	prev := orphanQueryMaxDeleted
	if scan {
		orphanQueryMaxDeleted = 0
	} else {
		orphanQueryMaxDeleted = 1 << 30
	}
	t.Cleanup(func() { orphanQueryMaxDeleted = prev })
}

func testRunOrphanedChildren(t *testing.T, scan bool) {
	iterDeleted := func(entries ...headstorage.HeadsEntry) func(ctx context.Context, opts headstorage.IterOpts, iter headstorage.EntryIterator) error {
		return func(ctx context.Context, opts headstorage.IterOpts, iter headstorage.EntryIterator) error {
			for _, e := range entries {
				_, _ = iter(e)
			}
			return nil
		}
	}
	// expectChildren serves the entries with a parent through the path under test
	expectChildren := func(fx *fixture, deletedIds []string, children ...headstorage.HeadsEntry) {
		if scan {
			fx.headStorage.EXPECT().IterateEntriesWithParent(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, iter headstorage.EntryIterator) error {
					for _, e := range children {
						_, _ = iter(e)
					}
					return nil
				})
			return
		}
		for _, parentId := range deletedIds {
			var byParent []headstorage.HeadsEntry
			for _, e := range children {
				if e.ParentId == parentId {
					byParent = append(byParent, e)
				}
			}
			fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return(byParent, nil)
		}
	}

	t.Run("queues orphaned children of deleted parents", func(t *testing.T) {
		setOrphanScan(t, scan)
		fx := newFixture(t)
		defer fx.stop()
		parentId := "parentId"
		childId := "childId"
		// IterateEntries returns the parent as fully deleted
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(iterDeleted(headstorage.HeadsEntry{Id: parentId, DeletedStatus: headstorage.DeletedStatusDeleted}))
		// a non-deleted child of the deleted parent and a child of a live parent
		expectChildren(fx, []string{parentId},
			headstorage.HeadsEntry{Id: childId, ParentId: parentId, DeletedStatus: headstorage.DeletedStatusNotDeleted},
			headstorage.HeadsEntry{Id: "otherChildId", ParentId: "liveParentId", DeletedStatus: headstorage.DeletedStatusNotDeleted},
		)
		// Expect the child to be queued
		fx.headStorage.EXPECT().UpdateEntry(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, update headstorage.HeadsUpdate) error {
			require.Equal(t, childId, update.Id)
			require.Equal(t, headstorage.DeletedStatusQueued, *update.DeletedStatus)
			return nil
		})

		err := fx.delState.Run(context.Background())
		require.NoError(t, err)
		require.Contains(t, fx.delState.deleted, parentId)
		require.Contains(t, fx.delState.queued, childId)
		require.NotContains(t, fx.delState.queued, "otherChildId")
	})

	t.Run("skips already deleted children", func(t *testing.T) {
		setOrphanScan(t, scan)
		fx := newFixture(t)
		defer fx.stop()
		parentId := "parentId"
		childId := "childId"
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(iterDeleted(headstorage.HeadsEntry{Id: parentId, DeletedStatus: headstorage.DeletedStatusDeleted}))
		// Child is already deleted — should not be queued
		expectChildren(fx, []string{parentId},
			headstorage.HeadsEntry{Id: childId, ParentId: parentId, DeletedStatus: headstorage.DeletedStatusDeleted})
		// No UpdateEntry call expected

		err := fx.delState.Run(context.Background())
		require.NoError(t, err)
		require.NotContains(t, fx.delState.queued, childId)
	})

	t.Run("no children — no error", func(t *testing.T) {
		setOrphanScan(t, scan)
		fx := newFixture(t)
		defer fx.stop()
		parentId := "parentId"
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(iterDeleted(headstorage.HeadsEntry{Id: parentId, DeletedStatus: headstorage.DeletedStatusDeleted}))
		expectChildren(fx, []string{parentId})

		err := fx.delState.Run(context.Background())
		require.NoError(t, err)
	})

	t.Run("no deleted parents — no lookups", func(t *testing.T) {
		setOrphanScan(t, scan)
		fx := newFixture(t)
		defer fx.stop()
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(iterDeleted(headstorage.HeadsEntry{Id: "queuedId", DeletedStatus: headstorage.DeletedStatusQueued}))
		// No IterateEntriesWithParent or GetEntriesByParentId call expected

		err := fx.delState.Run(context.Background())
		require.NoError(t, err)
		require.Contains(t, fx.delState.queued, "queuedId")
	})

	t.Run("lookup error — no error", func(t *testing.T) {
		setOrphanScan(t, scan)
		fx := newFixture(t)
		defer fx.stop()
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(iterDeleted(headstorage.HeadsEntry{Id: "parentId", DeletedStatus: headstorage.DeletedStatusDeleted}))
		if scan {
			fx.headStorage.EXPECT().IterateEntriesWithParent(gomock.Any(), gomock.Any()).Return(fmt.Errorf("scan error"))
		} else {
			fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), "parentId").Return(nil, fmt.Errorf("query error"))
		}

		err := fx.delState.Run(context.Background())
		require.NoError(t, err)
	})

	t.Run("same children as a query per deleted parent", func(t *testing.T) {
		setOrphanScan(t, scan)
		ctx := context.Background()
		db, err := anystore.Open(ctx, filepath.Join(t.TempDir(), "test.db"), nil)
		require.NoError(t, err)
		defer db.Close()
		hs, err := headstorage.New(ctx, db)
		require.NoError(t, err)

		status := func(s headstorage.DeletedStatus) *headstorage.DeletedStatus { return &s }
		parent := func(id string) *string { return &id }
		for _, u := range []headstorage.HeadsUpdate{
			{Id: "deleted1", DeletedStatus: status(headstorage.DeletedStatusDeleted)},
			{Id: "deleted2", DeletedStatus: status(headstorage.DeletedStatusDeleted)},
			{Id: "queued", DeletedStatus: status(headstorage.DeletedStatusQueued)},
			{Id: "live"},
			{Id: "deleted1.a", ParentId: parent("deleted1")},
			{Id: "deleted1.b", ParentId: parent("deleted1"), DeletedStatus: status(headstorage.DeletedStatusNotDeleted)},
			{Id: "deleted1.c", ParentId: parent("deleted1"), DeletedStatus: status(headstorage.DeletedStatusQueued)},
			{Id: "deleted2.a", ParentId: parent("deleted2")},
			{Id: "deleted2.b", ParentId: parent("deleted2"), DeletedStatus: status(headstorage.DeletedStatusDeleted)},
			{Id: "queued.a", ParentId: parent("queued")},
			{Id: "live.a", ParentId: parent("live")},
			{Id: "missing.a", ParentId: parent("missing")},
		} {
			require.NoError(t, hs.UpdateEntry(ctx, u))
		}

		// what the per-parent queries select
		var expected []string
		for _, parentId := range []string{"deleted1", "deleted2"} {
			children, err := hs.GetEntriesByParentId(ctx, parentId)
			require.NoError(t, err)
			for _, child := range children {
				if child.DeletedStatus == headstorage.DeletedStatusNotDeleted {
					expected = append(expected, child.Id)
				}
			}
		}
		sort.Strings(expected)
		require.Equal(t, []string{"deleted1.a", "deleted1.b", "deleted2.a"}, expected)

		delState := New().(*objectDeletionState)
		delState.storage = hs
		require.NoError(t, delState.Run(ctx))

		var orphans []string
		for id := range delState.queued {
			if id != "queued" && id != "deleted1.c" {
				orphans = append(orphans, id)
			}
		}
		sort.Strings(orphans)
		require.Equal(t, expected, orphans)
		for _, id := range expected {
			entry, err := hs.GetEntry(ctx, id)
			require.NoError(t, err)
			require.Equal(t, headstorage.DeletedStatusQueued, entry.DeletedStatus)
		}
	})
}

func TestDeletionState_Run_OrphanScanThreshold(t *testing.T) {
	prev := orphanQueryMaxDeleted
	orphanQueryMaxDeleted = 2
	defer func() { orphanQueryMaxDeleted = prev }()

	deleted := func(ids ...string) func(ctx context.Context, opts headstorage.IterOpts, iter headstorage.EntryIterator) error {
		return func(ctx context.Context, opts headstorage.IterOpts, iter headstorage.EntryIterator) error {
			for _, id := range ids {
				_, _ = iter(headstorage.HeadsEntry{Id: id, DeletedStatus: headstorage.DeletedStatusDeleted})
			}
			return nil
		}
	}

	t.Run("at the threshold — query per id", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(deleted("d1", "d2"))
		fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), "d1").Return(nil, nil)
		fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), "d2").Return(nil, nil)
		require.NoError(t, fx.delState.Run(context.Background()))
	})
	t.Run("above the threshold — one scan", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(deleted("d1", "d2", "d3"))
		fx.headStorage.EXPECT().IterateEntriesWithParent(gomock.Any(), gomock.Any()).Return(nil)
		require.NoError(t, fx.delState.Run(context.Background()))
	})
}

func TestDeletionState_Exists(t *testing.T) {
	fx := newFixture(t)
	defer fx.stop()

	fx.delState.queued["id1"] = struct{}{}
	fx.delState.deleted["id2"] = struct{}{}
	require.True(t, fx.delState.Exists("id1"))
	require.True(t, fx.delState.Exists("id2"))
	require.False(t, fx.delState.Exists("id3"))
}
