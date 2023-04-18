package settings

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/treemanager/mock_treemanager"
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
	treeManager := mock_treemanager.NewMockTreeManager(ctrl)

	delState.EXPECT().Add(state.DeletedIds).Return(nil)

	delManager := newDeletionManager(spaceId,
		settingsId,
		false,
		treeManager,
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
	treeManager := mock_treemanager.NewMockTreeManager(ctrl)
	provider := mock_settings.NewMockSpaceIdsProvider(ctrl)

	delState.EXPECT().Add(state.DeletedIds).Return(nil)
	provider.EXPECT().AllIds().Return([]string{"id", "otherId", settingsId})
	delState.EXPECT().Add([]string{"id", "otherId"}).Return(nil)
	delManager := newDeletionManager(spaceId,
		settingsId,
		true,
		treeManager,
		delState,
		provider,
		onDeleted)

	err := delManager.UpdateState(ctx, state)
	require.NoError(t, err)
	require.True(t, deleted)
}
