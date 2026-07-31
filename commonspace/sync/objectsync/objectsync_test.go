package objectsync

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces/mock_kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/object/treesyncer"
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
	t.Run("handle head update new proto, return request", func(t *testing.T) {
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
	t.Run("handle head update object missing, return request", func(t *testing.T) {
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
	t.Run("handle head update object missing, pull filter declines, update swallowed", func(t *testing.T) {
		root := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("rootChange"),
			Id:        "objectId",
		}
		var gotRoot *treechangeproto.RawTreeChangeWithId
		var gotHeads []string
		fx := newFixtureWithPullFilter(t, func(_ context.Context, objectId string, rootChange *treechangeproto.RawTreeChangeWithId, heads []string) bool {
			require.Equal(t, "objectId", objectId)
			gotRoot = rootChange
			gotHeads = heads
			return false
		})
		defer fx.close(t)
		update := headUpdateWithRoot(t, root, []string{"headId"})
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		r, err := fx.objectSync.HandleHeadUpdate(ctx, update)
		require.NoError(t, err)
		require.Nil(t, r)
		require.Equal(t, root.Id, gotRoot.Id)
		require.Equal(t, root.RawChange, gotRoot.RawChange)
		require.Equal(t, []string{"headId"}, gotHeads)
	})
	t.Run("handle head update object missing, pull filter accepts, return request", func(t *testing.T) {
		root := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("rootChange"),
			Id:        "objectId",
		}
		fx := newFixtureWithPullFilter(t, func(_ context.Context, _ string, _ *treechangeproto.RawTreeChangeWithId, _ []string) bool {
			return true
		})
		defer fx.close(t)
		update := headUpdateWithRoot(t, root, []string{"headId"})
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		r, err := fx.objectSync.HandleHeadUpdate(ctx, update)
		require.NoError(t, err)
		req := synctree.NewRequest("peerId", "spaceId", "objectId", nil, nil, nil)
		require.Equal(t, req, r)
	})
	t.Run("handle head update object missing, no root in update, pull filter skipped, return request", func(t *testing.T) {
		fx := newFixtureWithPullFilter(t, func(_ context.Context, _ string, _ *treechangeproto.RawTreeChangeWithId, _ []string) bool {
			require.Fail(t, "filter must not be consulted without a root change")
			return false
		})
		defer fx.close(t)
		update := &objectmessages.HeadUpdate{
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		fx.objectManager.EXPECT().GetObject(context.Background(), "objectId").Return(nil, fmt.Errorf("no object"))
		r, err := fx.objectSync.HandleHeadUpdate(ctx, update)
		require.NoError(t, err)
		req := synctree.NewRequest("peerId", "spaceId", "objectId", nil, nil, nil)
		require.Equal(t, req, r)
	})
}

func headUpdateWithRoot(t *testing.T, root *treechangeproto.RawTreeChangeWithId, heads []string) *objectmessages.HeadUpdate {
	treeHeadUpdate := &treechangeproto.TreeHeadUpdate{
		Heads:        heads,
		SnapshotPath: []string{root.Id},
	}
	wrapped := treechangeproto.WrapHeadUpdate(treeHeadUpdate, root)
	marshaled, err := wrapped.MarshalVT()
	require.NoError(t, err)
	return &objectmessages.HeadUpdate{
		Bytes: marshaled,
		Meta: objectmessages.ObjectMeta{
			PeerId:   "peerId",
			ObjectId: "objectId",
			SpaceId:  "spaceId",
		},
	}
}

func TestObjectSync_HandleStreamRequest(t *testing.T) {
	t.Run("handle stream request, object not found, return error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		fullSyncReq := &treechangeproto.TreeFullSyncRequest{
			Heads: []string{"headId"},
		}
		rootCh := &treechangeproto.RawTreeChangeWithId{
			Id: "objectId",
		}
		treeSyncMsg := treechangeproto.WrapFullRequest(fullSyncReq, rootCh)
		payload, err := treeSyncMsg.MarshalVT()
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
	t.Run("handle stream request, object found, handle on object level, return no request", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.close(t)
		fullSyncReq := &treechangeproto.TreeFullSyncRequest{
			Heads: []string{"headId"},
		}
		rootCh := &treechangeproto.RawTreeChangeWithId{
			Id: "objectId",
		}
		treeSyncMsg := treechangeproto.WrapFullRequest(fullSyncReq, rootCh)
		payload, err := treeSyncMsg.MarshalVT()
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
	t.Run("apply request, no error", func(t *testing.T) {
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
	t.Run("apply request, manager error", func(t *testing.T) {
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
	keyValue      *mock_kvinterfaces.MockKeyValueService
	pool          *mock_pool.MockService
	a             *app.App
	ctrl          *gomock.Controller
}

func newFixture(t *testing.T) *fixture {
	return newFixtureWithPullFilter(t, nil)
}

// filteringTreeSyncer is a minimal TreeSyncer that also implements the
// optional treesyncer.PullFilter extension.
type filteringTreeSyncer struct {
	shouldPull func(ctx context.Context, objectId string, rootChange *treechangeproto.RawTreeChangeWithId, heads []string) bool
}

func (f *filteringTreeSyncer) Init(a *app.App) error           { return nil }
func (f *filteringTreeSyncer) Name() string                    { return treesyncer.CName }
func (f *filteringTreeSyncer) Run(ctx context.Context) error   { return nil }
func (f *filteringTreeSyncer) Close(ctx context.Context) error { return nil }
func (f *filteringTreeSyncer) StartSync()                      {}
func (f *filteringTreeSyncer) StopSync()                       {}
func (f *filteringTreeSyncer) ShouldSync(string) bool          { return true }
func (f *filteringTreeSyncer) SyncAll(ctx context.Context, p peer.Peer, existing, missing []string) error {
	return nil
}

func (f *filteringTreeSyncer) ShouldPull(ctx context.Context, objectId string, rootChange *treechangeproto.RawTreeChangeWithId, heads []string) bool {
	return f.shouldPull(ctx, objectId, rootChange, heads)
}

func newFixtureWithPullFilter(t *testing.T, shouldPull func(ctx context.Context, objectId string, rootChange *treechangeproto.RawTreeChangeWithId, heads []string) bool) *fixture {
	fx := &fixture{
		a: &app.App{},
	}
	fx.ctrl = gomock.NewController(t)
	fx.objectManager = mock_objectmanager.NewMockObjectManager(fx.ctrl)
	fx.pool = mock_pool.NewMockService(fx.ctrl)
	fx.keyValue = mock_kvinterfaces.NewMockKeyValueService(fx.ctrl)
	anymock.ExpectComp(fx.objectManager.EXPECT(), treemanager.CName)
	anymock.ExpectComp(fx.pool.EXPECT(), pool.CName)
	anymock.ExpectComp(fx.keyValue.EXPECT(), kvinterfaces.CName)
	fx.objectSync = &objectSync{}
	spaceState := &spacestate.SpaceState{SpaceId: "spaceId"}
	fx.a.Register(fx.objectManager).
		Register(spaceState).
		Register(fx.pool).
		Register(fx.keyValue).
		Register(syncstatus.NewNoOpSyncStatus()).
		Register(fx.objectSync)
	if shouldPull != nil {
		fx.a.Register(&filteringTreeSyncer{shouldPull: shouldPull})
	}
	require.NoError(t, fx.a.Start(context.Background()))
	return fx
}

func (fx *fixture) close(t *testing.T) {
	err := fx.a.Close(context.Background())
	require.NoError(t, err)
	fx.ctrl.Finish()
}
