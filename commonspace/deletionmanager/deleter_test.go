package deletionmanager

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/deletionstate/mock_deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/object/treemanager/mock_treemanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
)

func TestDeleter_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	treeManager := mock_treemanager.NewMockTreeManager(ctrl)
	st := mock_spacestorage.NewMockSpaceStorage(ctrl)
	delState := mock_deletionstate.NewMockObjectDeletionState(ctrl)

	deleter := newDeleter(st, delState, treeManager, log)

	t.Run("deleter delete mark deleted success", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		st.EXPECT().TreeStorage(gomock.Any(), id).Return(nil, treestorage.ErrUnknownTreeId)
		treeManager.EXPECT().MarkTreeDeleted(gomock.Any(), spaceId, id).Return(nil)
		delState.EXPECT().Delete(id).Return(nil)

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
}
