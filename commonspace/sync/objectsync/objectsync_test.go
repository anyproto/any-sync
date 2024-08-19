package objectsync

import (
	"context"
	"fmt"
	"testing"

	"github.com/anyproto/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objectmanager/mock_objectmanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
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

func TestObjectSync_DeprecatedObjectSync(t *testing.T) {
	t.Run("handle deprecated sync empty message returned", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		fullSyncReq := &treechangeproto.TreeFullSyncRequest{}
		rootCh := &treechangeproto.RawTreeChangeWithId{
			Id: "objectId",
		}
		treeSyncMsg := treechangeproto.WrapFullRequest(fullSyncReq, rootCh)
		payload, err := treeSyncMsg.Marshal()
		require.NoError(t, err)
		msg := &spacesyncproto.ObjectSyncMessage{
			SpaceId:  "spaceId",
			Payload:  payload,
			ObjectId: "objectId",
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		ctx = peer.CtxWithProtoVersion(ctx, secureservice.ProtoVersion)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		r, err := fx.objectSync.HandleDeprecatedObjectSync(ctx, msg)
		require.NoError(t, err)
		retMsg := &spacesyncproto.ObjectSyncMessage{
			SpaceId:  "spaceId",
			ObjectId: "objectId",
		}
		require.Equal(t, retMsg, r)
	})
	t.Run("handle deprecated sync put tree", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		rootCh := &treechangeproto.RawTreeChangeWithId{
			Id: "objectId",
		}
		fullSyncReq := &treechangeproto.TreeFullSyncRequest{
			Changes: []*treechangeproto.RawTreeChangeWithId{
				rootCh,
			},
			Heads:        []string{"headId"},
			SnapshotPath: []string{"objectId"},
		}
		treeSyncMsg := treechangeproto.WrapFullRequest(fullSyncReq, rootCh)
		payload, err := treeSyncMsg.Marshal()
		require.NoError(t, err)
		msg := &spacesyncproto.ObjectSyncMessage{
			SpaceId:  "spaceId",
			Payload:  payload,
			ObjectId: "objectId",
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		ctx = peer.CtxWithProtoVersion(ctx, secureservice.ProtoVersion)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		fx.objectManager.EXPECT().ValidateAndPutTree(ctx, "spaceId", gomock.Any()).Return(nil)
		retResp := &treechangeproto.TreeFullSyncResponse{
			Heads:        []string{"headId"},
			SnapshotPath: []string{"objectId"},
		}
		retWrappedResp := treechangeproto.WrapFullResponse(retResp, rootCh)
		returnMarshaled, err := retWrappedResp.Marshal()
		require.NoError(t, err)
		returnPayload := &spacesyncproto.ObjectSyncMessage{
			SpaceId:  "spaceId",
			Payload:  returnMarshaled,
			ObjectId: "objectId",
		}
		r, err := fx.objectSync.HandleDeprecatedObjectSync(ctx, msg)
		require.NoError(t, err)
		retMsg := &spacesyncproto.ObjectSyncMessage{
			SpaceId:  "spaceId",
			ObjectId: "objectId",
			Payload:  returnPayload.Payload,
		}
		require.Equal(t, retMsg, r)
	})
	t.Run("handle deprecated sync ok", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		fullSyncReq := &treechangeproto.TreeFullSyncRequest{}
		rootCh := &treechangeproto.RawTreeChangeWithId{
			Id: "objectId",
		}
		treeSyncMsg := treechangeproto.WrapFullRequest(fullSyncReq, rootCh)
		payload, err := treeSyncMsg.Marshal()
		require.NoError(t, err)
		msg := &spacesyncproto.ObjectSyncMessage{
			SpaceId:  "spaceId",
			Payload:  payload,
			ObjectId: "objectId",
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		ctx = peer.CtxWithProtoVersion(ctx, secureservice.CompatibleVersion)
		objHandler := mock_syncdeps.NewMockObjectSyncHandler(fx.ctrl)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(syncGetter{objHandler}, nil)
		objHandler.EXPECT().HandleDeprecatedRequest(ctx, msg).Return(&spacesyncproto.ObjectSyncMessage{}, nil)
		r, err := fx.objectSync.HandleDeprecatedObjectSync(ctx, msg)
		require.NoError(t, err)
		require.NotNil(t, r)
	})
}

func TestObjectSync_HandleStreamRequest(t *testing.T) {
	t.Run("handle stream request object not found", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		fullSyncReq := &treechangeproto.TreeFullSyncRequest{
			Heads: []string{"headId"},
		}
		rootCh := &treechangeproto.RawTreeChangeWithId{
			Id: "objectId",
		}
		treeSyncMsg := treechangeproto.WrapFullRequest(fullSyncReq, rootCh)
		payload, err := treeSyncMsg.Marshal()
		require.NoError(t, err)
		sendResponse := func(resp proto.Message) error {
			return nil
		}
		byteReq := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", payload)
		expectedReq := synctree.NewRequest("peerId", "spaceId", "objectId", nil, nil, nil)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		r, err := fx.objectSync.HandleStreamRequest(ctx, byteReq, nil, sendResponse)
		require.Equal(t, treechangeproto.ErrGetTree, err)
		require.Equal(t, expectedReq, r)
	})
	t.Run("handle stream request object not found", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		fullSyncReq := &treechangeproto.TreeFullSyncRequest{
			Heads: []string{"headId"},
		}
		rootCh := &treechangeproto.RawTreeChangeWithId{
			Id: "objectId",
		}
		treeSyncMsg := treechangeproto.WrapFullRequest(fullSyncReq, rootCh)
		payload, err := treeSyncMsg.Marshal()
		require.NoError(t, err)
		sendResponse := func(resp proto.Message) error {
			return nil
		}
		byteReq := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", payload)
		objHandler := mock_syncdeps.NewMockObjectSyncHandler(fx.ctrl)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(syncGetter{objHandler}, nil)
		objHandler.EXPECT().HandleStreamRequest(ctx, byteReq, nil, gomock.Any()).Return(nil, nil)
		r, err := fx.objectSync.HandleStreamRequest(ctx, byteReq, nil, sendResponse)
		require.NoError(t, err)
		require.Nil(t, r)
	})
}

func TestObjectSync_ApplyRequest(t *testing.T) {
	t.Run("apply request nil manager error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		requestSender := mock_syncdeps.NewMockRequestSender(fx.ctrl)
		objHandler := mock_syncdeps.NewMockObjectSyncHandler(fx.ctrl)
		responseCollector := mock_syncdeps.NewMockResponseCollector(fx.ctrl)

		rq := synctree.NewRequest("peerId", "spaceId", "objectId", nil, nil, nil)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(syncGetter{objHandler}, nil)
		objHandler.EXPECT().ResponseCollector().Return(responseCollector)
		peerCtx := peer.CtxWithPeerId(ctx, "peerId")
		requestSender.EXPECT().SendRequest(peerCtx, rq, responseCollector).Return(nil)
		err := fx.objectSync.ApplyRequest(ctx, rq, requestSender)
		require.NoError(t, err)
	})
	t.Run("apply request manager error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		requestSender := mock_syncdeps.NewMockRequestSender(fx.ctrl)
		rq := synctree.NewRequest("peerId", "spaceId", "objectId", nil, nil, nil)
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		peerCtx := peer.CtxWithPeerId(ctx, "peerId")
		fx.objectManager.EXPECT().GetObject(peerCtx, "objectId").Return(nil, nil)
		err := fx.objectSync.ApplyRequest(ctx, rq, requestSender)
		require.NoError(t, err)
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
