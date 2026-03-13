package deletionstate

import (
	"context"
	"sort"
	"testing"

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
	t.Run("queues orphaned children of deleted parents", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		parentId := "parentId"
		childId := "childId"
		// IterateEntries returns the parent as fully deleted
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, iter headstorage.EntryIterator) error {
				_, _ = iter(headstorage.HeadsEntry{Id: parentId, DeletedStatus: headstorage.DeletedStatusDeleted})
				return nil
			})
		// GetEntriesByParentId returns a non-deleted child
		fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return([]headstorage.HeadsEntry{
			{Id: childId, DeletedStatus: headstorage.DeletedStatusNotDeleted},
		}, nil)
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
	})

	t.Run("skips already deleted children", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		parentId := "parentId"
		childId := "childId"
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, iter headstorage.EntryIterator) error {
				_, _ = iter(headstorage.HeadsEntry{Id: parentId, DeletedStatus: headstorage.DeletedStatusDeleted})
				return nil
			})
		// Child is already deleted — should not be queued
		fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return([]headstorage.HeadsEntry{
			{Id: childId, DeletedStatus: headstorage.DeletedStatusDeleted},
		}, nil)
		// No UpdateEntry call expected

		err := fx.delState.Run(context.Background())
		require.NoError(t, err)
		require.NotContains(t, fx.delState.queued, childId)
	})

	t.Run("no children — no error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		parentId := "parentId"
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), headstorage.IterOpts{Deleted: true}, gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, iter headstorage.EntryIterator) error {
				_, _ = iter(headstorage.HeadsEntry{Id: parentId, DeletedStatus: headstorage.DeletedStatusDeleted})
				return nil
			})
		fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return(nil, nil)

		err := fx.delState.Run(context.Background())
		require.NoError(t, err)
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
