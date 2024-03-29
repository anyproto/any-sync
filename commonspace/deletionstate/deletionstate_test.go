package deletionstate

import (
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"sort"
	"testing"
)

type fixture struct {
	ctrl         *gomock.Controller
	delState     *objectDeletionState
	spaceStorage *mock_spacestorage.MockSpaceStorage
}

func newFixture(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	spaceStorage := mock_spacestorage.NewMockSpaceStorage(ctrl)
	delState := New().(*objectDeletionState)
	delState.storage = spaceStorage
	return &fixture{
		ctrl:         ctrl,
		delState:     delState,
		spaceStorage: spaceStorage,
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
		fx.spaceStorage.EXPECT().TreeDeletedStatus(id).Return("", nil)
		fx.spaceStorage.EXPECT().SetTreeDeletedStatus(id, spacestorage.TreeDeletedStatusQueued).Return(nil)
		fx.delState.Add(map[string]struct{}{id: {}})
		require.Contains(t, fx.delState.queued, id)
	})

	t.Run("add existing queued", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		id := "newId"
		fx.spaceStorage.EXPECT().TreeDeletedStatus(id).Return(spacestorage.TreeDeletedStatusQueued, nil)
		fx.delState.Add(map[string]struct{}{id: {}})
		require.Contains(t, fx.delState.queued, id)
	})

	t.Run("add existing deleted", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		id := "newId"
		fx.spaceStorage.EXPECT().TreeDeletedStatus(id).Return(spacestorage.TreeDeletedStatusDeleted, nil)
		fx.delState.Add(map[string]struct{}{id: {}})
		require.Contains(t, fx.delState.deleted, id)
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

func TestDeletionState_AddObserver(t *testing.T) {
	fx := newFixture(t)
	defer fx.stop()

	var queued []string

	fx.delState.AddObserver(func(ids []string) {
		queued = ids
	})
	id := "newId"
	fx.spaceStorage.EXPECT().TreeDeletedStatus(id).Return("", nil)
	fx.spaceStorage.EXPECT().SetTreeDeletedStatus(id, spacestorage.TreeDeletedStatusQueued).Return(nil)
	fx.delState.Add(map[string]struct{}{id: {}})
	require.Contains(t, fx.delState.queued, id)
	require.Equal(t, []string{id}, queued)
}

func TestDeletionState_Delete(t *testing.T) {
	fx := newFixture(t)
	defer fx.stop()

	id := "deletedId"
	fx.delState.queued[id] = struct{}{}
	fx.spaceStorage.EXPECT().SetTreeDeletedStatus(id, spacestorage.TreeDeletedStatusDeleted).Return(nil)
	err := fx.delState.Delete(id)
	require.NoError(t, err)
	require.Contains(t, fx.delState.deleted, id)
	require.NotContains(t, fx.delState.queued, id)
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
