package settings

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/deletionstate/mock_deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/treemanager/mock_treemanager"
	"github.com/anyproto/any-sync/commonspace/settings/mock_settings"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestDeletionManager_UpdateState_NotResponsible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	spaceId := "spaceId"
	settingsId := "settingsId"
	state := &settingsstate.State{
		DeletedIds: map[string]struct{}{"id": {}},
		DeleterId:  "deleterId",
	}
	deleted := false
	onDeleted := func() {
		deleted = true
	}
	delState := mock_deletionstate.NewMockObjectDeletionState(ctrl)
	treeManager := mock_treemanager.NewMockTreeManager(ctrl)

	delState.EXPECT().Add(state.DeletedIds)

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
		DeletedIds: map[string]struct{}{"id": struct{}{}},
		DeleterId:  "deleterId",
	}
	deleted := false
	onDeleted := func() {
		deleted = true
	}
	delState := mock_deletionstate.NewMockObjectDeletionState(ctrl)
	treeManager := mock_treemanager.NewMockTreeManager(ctrl)
	provider := mock_settings.NewMockSpaceIdsProvider(ctrl)

	delState.EXPECT().Add(state.DeletedIds)
	provider.EXPECT().AllIds().Return([]string{"id", "otherId", settingsId})
	delState.EXPECT().Add(map[string]struct{}{"id": {}, "otherId": {}})
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
