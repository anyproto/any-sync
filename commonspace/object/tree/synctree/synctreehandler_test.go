package synctree

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/secureservice"
)

type testObjTreeMock struct {
	*mock_objecttree.MockObjectTree
	m sync.RWMutex
}

func (t *testObjTreeMock) AddRawChangesFromPeer(ctx context.Context, peerId string, changesPayload objecttree.RawChangesPayload) (res objecttree.AddResult, err error) {
	return t.MockObjectTree.AddRawChanges(ctx, changesPayload)
}

func newTestObjMock(mockTree *mock_objecttree.MockObjectTree) *testObjTreeMock {
	return &testObjTreeMock{
		MockObjectTree: mockTree,
	}
}

func (t *testObjTreeMock) Lock() {
	t.m.Lock()
}

func (t *testObjTreeMock) RLock() {
	t.m.RLock()
}

func (t *testObjTreeMock) Unlock() {
	t.m.Unlock()
}

func (t *testObjTreeMock) RUnlock() {
	t.m.RUnlock()
}

func (t *testObjTreeMock) TryLock() bool {
	return t.m.TryLock()
}

func (t *testObjTreeMock) TryRLock() bool {
	return t.m.TryRLock()
}

type syncHandlerFixture struct {
	ctrl             *gomock.Controller
	syncClientMock   *mock_synctree.MockSyncClient
	objectTreeMock   *testObjTreeMock
	syncProtocolMock *mock_synctree.MockTreeSyncProtocol
	spaceId          string
	senderId         string
	treeId           string

	syncHandler *syncTreeHandler
}

func newSyncHandlerFixture(t *testing.T) *syncHandlerFixture {
	ctrl := gomock.NewController(t)
	objectTreeMock := newTestObjMock(mock_objecttree.NewMockObjectTree(ctrl))
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	syncProtocolMock := mock_synctree.NewMockTreeSyncProtocol(ctrl)
	spaceId := "spaceId"

	syncHandler := &syncTreeHandler{
		objTree:         objectTreeMock,
		syncClient:      syncClientMock,
		syncProtocol:    syncProtocolMock,
		spaceId:         spaceId,
		syncStatus:      syncstatus.NewNoOpSyncStatus(),
		pendingRequests: map[string]struct{}{},
	}
	return &syncHandlerFixture{
		ctrl:             ctrl,
		objectTreeMock:   objectTreeMock,
		syncProtocolMock: syncProtocolMock,
		syncClientMock:   syncClientMock,
		syncHandler:      syncHandler,
		spaceId:          spaceId,
		senderId:         "senderId",
		treeId:           "treeId",
	}
}

func (fx *syncHandlerFixture) stop() {
	fx.ctrl.Finish()
}

func TestSyncTreeHandler_HandleMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("handle head update message, heads not equal, request returned", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads: []string{"h3"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		syncReq := &treechangeproto.TreeSyncMessage{}
		fx.syncHandler.heads = []string{"h2"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Times(2).Return([]string{"h2"})
		fx.objectTreeMock.EXPECT().Heads().Times(2).Return([]string{"h3"})
		fx.syncProtocolMock.EXPECT().HeadUpdate(ctx, fx.senderId, uint32(0), gomock.Any()).Return(syncReq, nil)
		fx.syncClientMock.EXPECT().QueueRequest(fx.senderId, fx.treeId, syncReq).Return(nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, 0, objectMsg)
		require.NoError(t, err)
		require.Equal(t, []string{"h3"}, fx.syncHandler.heads)
	})

	t.Run("handle head update message, heads equal", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		fx.syncHandler.heads = []string{"h1"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, 0, objectMsg)
		require.NoError(t, err)
	})

	t.Run("handle head update message, no sync request returned", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads: []string{"h3"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		fx.syncHandler.heads = []string{"h2"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Times(2).Return([]string{"h2"})
		fx.objectTreeMock.EXPECT().Heads().Times(2).Return([]string{"h3"})
		fx.syncProtocolMock.EXPECT().HeadUpdate(ctx, fx.senderId, uint32(0), gomock.Any()).Return(nil, nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, 0, objectMsg)
		require.NoError(t, err)
		require.Equal(t, []string{"h3"}, fx.syncHandler.heads)
	})

	t.Run("handle full sync request returns error", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads: []string{"h3"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		fx.syncHandler.heads = []string{"h2"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Times(3).Return([]string{"h2"})

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, 0, objectMsg)
		require.Equal(t, err, ErrMessageIsRequest)
	})

	t.Run("handle full sync response", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{
			Heads: []string{"h3"},
		}
		treeMsg := treechangeproto.WrapFullResponse(fullSyncResponse, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		fx.syncHandler.heads = []string{"h2"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Times(2).Return([]string{"h2"})
		fx.objectTreeMock.EXPECT().Heads().Times(2).Return([]string{"h3"})
		fx.syncProtocolMock.EXPECT().FullSyncResponse(ctx, fx.senderId, gomock.Any()).Return(nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, 0, objectMsg)
		require.NoError(t, err)
	})

	t.Run("handle full sync response new protocol heads not equal", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{
			Heads:        []string{"h3"},
			SnapshotPath: []string{"h3"},
		}
		treeMsg := treechangeproto.WrapFullResponse(fullSyncResponse, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		fx.syncHandler.heads = []string{"h2"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"h2"})
		fx.syncProtocolMock.EXPECT().FullSyncResponse(ctx, fx.senderId, gomock.Any()).Return(nil)
		req := &treechangeproto.TreeSyncMessage{}
		fx.syncClientMock.EXPECT().CreateFullSyncRequest(fx.objectTreeMock, fullSyncResponse.Heads, fullSyncResponse.SnapshotPath).Return(req, nil)
		fx.syncClientMock.EXPECT().QueueRequest(fx.senderId, fx.treeId, req).Return(nil)
		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, secureservice.NewSyncProtoVersion, objectMsg)
		require.NoError(t, err)
	})

	t.Run("handle full sync response new protocol heads equal", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{
			Heads:        []string{"h3"},
			SnapshotPath: []string{"h3"},
		}
		treeMsg := treechangeproto.WrapFullResponse(fullSyncResponse, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		fx.syncHandler.heads = []string{"h2"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Times(2).Return([]string{"h2"})
		fx.objectTreeMock.EXPECT().Heads().Times(3).Return([]string{"h3"})
		fx.syncProtocolMock.EXPECT().FullSyncResponse(ctx, fx.senderId, gomock.Any()).Return(nil)
		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, secureservice.NewSyncProtoVersion, objectMsg)
		require.NoError(t, err)
	})

	t.Run("handle full sync response new protocol empty message", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		objectMsg := &spacesyncproto.ObjectSyncMessage{ObjectId: treeId}
		fx.syncHandler.heads = []string{"h2"}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"h2"})
		req := &treechangeproto.TreeSyncMessage{}
		fx.syncClientMock.EXPECT().CreateFullSyncRequest(fx.objectTreeMock, nil, nil).Return(req, nil)
		fx.syncClientMock.EXPECT().QueueRequest(fx.senderId, fx.treeId, req).Return(nil)
		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, secureservice.NewSyncProtoVersion, objectMsg)
		require.NoError(t, err)
	})
}

func TestSyncTreeHandler_HandleRequest(t *testing.T) {
	ctx := context.Background()

	t.Run("handle request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullRequest := &treechangeproto.TreeFullSyncRequest{}
		treeMsg := treechangeproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(treeMsg, "spaceId", treeId)

		syncResp := &treechangeproto.TreeSyncMessage{}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.syncProtocolMock.EXPECT().FullSyncRequest(ctx, fx.senderId, gomock.Any()).Return(syncResp, nil)

		res, err := fx.syncHandler.HandleRequest(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("handle other message", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullResponse := &treechangeproto.TreeFullSyncResponse{}
		responseMsg := treechangeproto.WrapFullResponse(fullResponse, chWithId)
		headUpdate := &treechangeproto.TreeHeadUpdate{}
		headUpdateMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		for _, msg := range []*treechangeproto.TreeSyncMessage{responseMsg, headUpdateMsg} {
			objectMsg, _ := spacesyncproto.MarshallSyncMessage(msg, "spaceId", treeId)

			_, err := fx.syncHandler.HandleRequest(ctx, fx.senderId, objectMsg)
			require.Equal(t, err, ErrMessageIsNotRequest)
		}
	})
}
