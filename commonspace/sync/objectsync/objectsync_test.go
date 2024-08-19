package objectsync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objectmanager/mock_objectmanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps/mock_syncdeps"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/pool/mock_pool"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/testutil/anymock"
)

var ctx = context.Background()

type syncGetter struct {
	syncdeps.ObjectSyncHandler
}

func (s syncGetter) Id() string {
	return "objectId"
}

func TestObjectSync_HandleHeadUpdate(t *testing.T) {
	t.Run("handle head update new proto ok", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		update := &objectmessages.HeadUpdate{
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		ctx = peer.CtxWithProtoVersion(ctx, secureservice.ProtoVersion)
		objHandler := mock_syncdeps.NewMockObjectSyncHandler(fx.ctrl)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(syncGetter{objHandler}, nil)
		req := &objectmessages.Request{
			Bytes: []byte("byte"),
		}
		objHandler.EXPECT().HandleHeadUpdate(ctx, fx.status, update).Return(req, nil)
		r, err := fx.objectSync.HandleHeadUpdate(ctx, update)
		require.NoError(t, err)
		require.Equal(t, req, r)
	})
	t.Run("handle head update old proto ok", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		update := &objectmessages.HeadUpdate{
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		ctx = peer.CtxWithProtoVersion(ctx, secureservice.CompatibleVersion)
		objHandler := mock_syncdeps.NewMockObjectSyncHandler(fx.ctrl)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(syncGetter{objHandler}, nil)
		req := &objectmessages.Request{
			Bytes: []byte("byte"),
		}
		objHandler.EXPECT().HandleHeadUpdate(ctx, fx.status, update).Return(req, nil)
		r, err := fx.objectSync.HandleHeadUpdate(ctx, update)
		require.NoError(t, err)
		require.Nil(t, r)
	})
	t.Run("handle head update object missing", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		update := &objectmessages.HeadUpdate{
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		ctx = peer.CtxWithProtoVersion(ctx, secureservice.ProtoVersion)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		r, err := fx.objectSync.HandleHeadUpdate(ctx, update)
		require.NoError(t, err)
		req := synctree.NewRequest(update.Meta.PeerId, update.Meta.SpaceId, update.Meta.ObjectId, nil, nil, nil)
		require.Equal(t, req, r)
	})
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
