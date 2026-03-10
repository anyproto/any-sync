package deletionmanager

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/deletionstate/mock_deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage/mock_headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/object/treemanager/mock_treemanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
)

func TestDeleter_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	treeManager := mock_treemanager.NewMockTreeManager(ctrl)
	st := mock_spacestorage.NewMockSpaceStorage(ctrl)
	delState := mock_deletionstate.NewMockObjectDeletionState(ctrl)
	headStorage := mock_headstorage.NewMockHeadStorage(ctrl)

	deleter := newDeleter(st, delState, treeManager, log)

	t.Run("deleter delete mark deleted success", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), id).Return(nil, treestorage.ErrUnknownTreeId)
		treeManager.EXPECT().MarkTreeDeleted(gomock.Any(), spaceId, id).Return(nil)
		delState.EXPECT().Delete(id).Return(nil)
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), id).Return(nil, nil)

		deleter.Delete(context.Background())
	})

	t.Run("deleter delete mark deleted other error", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), id).Return(nil, fmt.Errorf("unknown error"))

		deleter.Delete(context.Background())
	})

	t.Run("deleter delete mark deleted fail", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), id).Return(nil, treestorage.ErrUnknownTreeId)
		treeManager.EXPECT().MarkTreeDeleted(gomock.Any(), spaceId, id).Return(fmt.Errorf("mark error"))

		deleter.Delete(context.Background())
	})

	t.Run("deleter delete success", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), id).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, id).Return(nil)
		delState.EXPECT().Delete(id).Return(nil)
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), id).Return(nil, nil)

		deleter.Delete(context.Background())
	})

	t.Run("deleter delete error", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), id).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, id).Return(fmt.Errorf("some error"))

		deleter.Delete(context.Background())
	})

	t.Run("delete bound children happy path", func(t *testing.T) {
		parentId := "parent"
		childId := "child1"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{parentId})
		st.EXPECT().Id().Return(spaceId)
		// parent deletion
		st.EXPECT().TreeStorage(gomock.Any(), parentId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, parentId).Return(nil)
		delState.EXPECT().Delete(parentId).Return(nil)
		// children lookup
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return([]headstorage.HeadsEntry{
			{Id: childId, DeletedStatus: headstorage.DeletedStatusNotDeleted},
		}, nil)
		// child deletion
		st.EXPECT().TreeStorage(gomock.Any(), childId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, childId).Return(nil)
		delState.EXPECT().Delete(childId).Return(nil)

		deleter.Delete(context.Background())
	})

	t.Run("delete bound children skips already deleted", func(t *testing.T) {
		parentId := "parent"
		childId := "child-deleted"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{parentId})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), parentId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, parentId).Return(nil)
		delState.EXPECT().Delete(parentId).Return(nil)
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return([]headstorage.HeadsEntry{
			{Id: childId, DeletedStatus: headstorage.DeletedStatusDeleted},
		}, nil)
		// no child deletion calls expected

		deleter.Delete(context.Background())
	})

	t.Run("delete bound children GetEntriesByParentId error", func(t *testing.T) {
		parentId := "parent"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{parentId})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), parentId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, parentId).Return(nil)
		delState.EXPECT().Delete(parentId).Return(nil)
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return(nil, fmt.Errorf("db error"))

		deleter.Delete(context.Background())
	})

	t.Run("delete bound children tryMarkDeleted failure", func(t *testing.T) {
		parentId := "parent"
		childId := "child-mark-fail"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{parentId})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), parentId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, parentId).Return(nil)
		delState.EXPECT().Delete(parentId).Return(nil)
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return([]headstorage.HeadsEntry{
			{Id: childId, DeletedStatus: headstorage.DeletedStatusNotDeleted},
		}, nil)
		// child tryMarkDeleted returns non-tree-storage error
		st.EXPECT().TreeStorage(gomock.Any(), childId).Return(nil, fmt.Errorf("unexpected error"))

		deleter.Delete(context.Background())
	})

	t.Run("delete bound children DeleteTree failure", func(t *testing.T) {
		parentId := "parent"
		childId := "child-del-fail"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{parentId})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), parentId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, parentId).Return(nil)
		delState.EXPECT().Delete(parentId).Return(nil)
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return([]headstorage.HeadsEntry{
			{Id: childId, DeletedStatus: headstorage.DeletedStatusNotDeleted},
		}, nil)
		st.EXPECT().TreeStorage(gomock.Any(), childId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, childId).Return(fmt.Errorf("delete tree error"))

		deleter.Delete(context.Background())
	})

	t.Run("delete bound children state.Delete failure", func(t *testing.T) {
		parentId := "parent"
		childId := "child-state-fail"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{parentId})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), parentId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, parentId).Return(nil)
		delState.EXPECT().Delete(parentId).Return(nil)
		st.EXPECT().HeadStorage().Return(headStorage)
		headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), parentId).Return([]headstorage.HeadsEntry{
			{Id: childId, DeletedStatus: headstorage.DeletedStatusNotDeleted},
		}, nil)
		st.EXPECT().TreeStorage(gomock.Any(), childId).Return(nil, nil)
		treeManager.EXPECT().DeleteTree(gomock.Any(), spaceId, childId).Return(nil)
		delState.EXPECT().Delete(childId).Return(fmt.Errorf("state delete error"))

		deleter.Delete(context.Background())
	})
}
