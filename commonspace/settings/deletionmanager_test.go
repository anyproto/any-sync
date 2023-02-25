package settings

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter/mock_treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/mock_settings"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate/mock_settingsstate"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDeletionManager_UpdateState_NotResponsible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	spaceId := "spaceId"
	settingsId := "settingsId"
	state := &settingsstate.State{
		DeletedIds: []string{"id"},
		DeleterId:  "deleterId",
	}
	deleted := false
	onDeleted := func() {
		deleted = true
	}
	delState := mock_settingsstate.NewMockObjectDeletionState(ctrl)
	treeGetter := mock_treegetter.NewMockTreeGetter(ctrl)

	delState.EXPECT().Add(state.DeletedIds).Return(nil)
	treeGetter.EXPECT().DeleteSpace(ctx, spaceId).Return(nil)

	delManager := newDeletionManager(spaceId,
		settingsId,
		false,
		treeGetter,
		delState,
		nil,
		onDeleted)
	err := delManager.UpdateState(ctx, state)
	require.NoError(t, err)
	require.True(t, deleted)
}

func TestDeletionManager_UpdateState_Responsible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	spaceId := "spaceId"
	settingsId := "settingsId"
	state := &settingsstate.State{
		DeletedIds: []string{"id"},
		DeleterId:  "deleterId",
	}
	deleted := false
	onDeleted := func() {
		deleted = true
	}
	delState := mock_settingsstate.NewMockObjectDeletionState(ctrl)
	treeGetter := mock_treegetter.NewMockTreeGetter(ctrl)
	provider := mock_settings.NewMockSpaceIdsProvider(ctrl)

	delState.EXPECT().Add(state.DeletedIds).Return(nil)
	treeGetter.EXPECT().DeleteSpace(ctx, spaceId).Return(nil)
	provider.EXPECT().AllIds().Return([]string{"id", "otherId", settingsId})
	delState.EXPECT().Add([]string{"id", "otherId"}).Return(nil)
	delManager := newDeletionManager(spaceId,
		settingsId,
		true,
		treeGetter,
		delState,
		provider,
		onDeleted)

	err := delManager.UpdateState(ctx, state)
	require.NoError(t, err)
	require.True(t, deleted)
}
