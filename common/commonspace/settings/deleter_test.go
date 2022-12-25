package settings

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings/deletionstate/mock_deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage/mock_storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter/mock_treegetter"
	"github.com/golang/mock/gomock"
	"testing"
)

func TestDeleter_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	treeGetter := mock_treegetter.NewMockTreeGetter(ctrl)
	st := mock_storage.NewMockSpaceStorage(ctrl)
	delState := mock_deletionstate.NewMockDeletionState(ctrl)

	deleter := newDeleter(st, delState, treeGetter)

	t.Run("deleter delete queued", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		treeGetter.EXPECT().DeleteTree(gomock.Any(), spaceId, id).Return(nil)
		delState.EXPECT().Delete(id).Return(nil)

		deleter.Delete()
	})

	t.Run("deleter delete already deleted", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		treeGetter.EXPECT().DeleteTree(gomock.Any(), spaceId, id).Return(storage.ErrTreeStorageAlreadyDeleted)
		delState.EXPECT().Delete(id).Return(nil)

		deleter.Delete()
	})

	t.Run("deleter delete error", func(t *testing.T) {
		id := "id"
		spaceId := "spaceId"
		delState.EXPECT().GetQueued().Return([]string{id})
		st.EXPECT().Id().Return(spaceId)
		treeGetter.EXPECT().DeleteTree(gomock.Any(), spaceId, id).Return(fmt.Errorf("some error"))

		deleter.Delete()
	})
}
