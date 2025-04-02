package deletionmanager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/deletionmanager/mock_deletionmanager"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/deletionstate/mock_deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/object/treemanager/mock_treemanager"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
)

type deletionManagerFixture struct {
	delState    *mock_deletionstate.MockObjectDeletionState
	treeManager *mock_treemanager.MockTreeManager
	storage     *mock_spacestorage.MockSpaceStorage
	deleter     *mock_deletionmanager.MockDeleter
	spaceState  *spacestate.SpaceState
	settingsId  string

	app        *app.App
	delManager *deletionManager

	ctrl *gomock.Controller
}

func newDelManagerFixture(t *testing.T) *deletionManagerFixture {
	ctrl := gomock.NewController(t)
	spaceState := &spacestate.SpaceState{
		SpaceId: "spaceId",
	}
	fx := &deletionManagerFixture{
		delState:    mock_deletionstate.NewMockObjectDeletionState(ctrl),
		treeManager: mock_treemanager.NewMockTreeManager(ctrl),
		storage:     mock_spacestorage.NewMockSpaceStorage(ctrl),
		deleter:     mock_deletionmanager.NewMockDeleter(ctrl),
		spaceState:  spaceState,
		settingsId:  "settingsId",
		delManager:  New().(*deletionManager),
		ctrl:        ctrl,
		app:         &app.App{},
	}
	return fx
}

func (fx *deletionManagerFixture) init(t *testing.T) {
	fx.delState.EXPECT().Name().AnyTimes().Return(deletionstate.CName)
	fx.treeManager.EXPECT().Name().AnyTimes().Return(treemanager.CName)
	fx.storage.EXPECT().Name().AnyTimes().Return(spacestorage.CName)
	fx.delState.EXPECT().AddObserver(gomock.Any())
	fx.app.Register(fx.spaceState).
		Register(fx.storage).
		Register(fx.delState).
		Register(fx.treeManager).
		Register(fx.delManager)

	err := fx.delManager.Init(fx.app)
	require.NoError(t, err)
	fx.delManager.deleter = fx.deleter
}

func (fx *deletionManagerFixture) stop() {
	fx.ctrl.Finish()
}

func TestDeletionManager_UpdateState(t *testing.T) {
	fx := newDelManagerFixture(t)
	fx.init(t)
	defer fx.stop()

	ctx := context.Background()
	state := &settingsstate.State{
		DeletedIds: map[string]struct{}{"id": {}},
	}
	fx.delState.EXPECT().Add(state.DeletedIds)
	err := fx.delManager.UpdateState(ctx, state)
	require.NoError(t, err)
}
