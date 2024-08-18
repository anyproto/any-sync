package objectsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objectmanager/mock_objectmanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/pool/mock_pool"
	"github.com/anyproto/any-sync/testutil/anymock"
)

func TestObjectSync_HandleHeadUpdate(t *testing.T) {
	fx := newFixture(t)
	defer fx.close(t)
}

type fixture struct {
	*objectSync
	objectManager *mock_objectmanager.MockObjectManager
	pool          *mock_pool.MockService
	a             *app.App
	ctrl          *gomock.Controller
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		a: &app.App{},
	}
	fx.ctrl = gomock.NewController(t)
	fx.objectManager = mock_objectmanager.NewMockObjectManager(fx.ctrl)
	fx.pool = mock_pool.NewMockService(fx.ctrl)
	anymock.ExpectComp(fx.objectManager.EXPECT(), treemanager.CName)
	anymock.ExpectComp(fx.pool.EXPECT(), pool.CName)
	fx.objectSync = &objectSync{}
	spaceState := &spacestate.SpaceState{SpaceId: "spaceId"}
	fx.a.Register(fx.objectManager).
		Register(spaceState).
		Register(fx.pool).
		Register(syncstatus.NewNoOpSyncStatus()).
		Register(fx.objectSync)
	require.NoError(t, fx.a.Start(context.Background()))
	return fx
}

func (fx *fixture) close(t *testing.T) {
	err := fx.a.Close(context.Background())
	require.NoError(t, err)
	fx.ctrl.Finish()
}
