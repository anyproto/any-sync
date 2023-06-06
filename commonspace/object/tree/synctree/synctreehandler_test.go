package synctree

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/golang/mock/gomock"
)

type testObjTreeMock struct {
	*mock_objecttree.MockObjectTree
	m sync.RWMutex
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
	receiveQueueMock ReceiveQueue
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
	receiveQueue := newReceiveQueue(5)

	syncHandler := &syncTreeHandler{
		objTree:      objectTreeMock,
		syncClient:   syncClientMock,
		syncProtocol: syncProtocolMock,
		spaceId:      spaceId,
		queue:        receiveQueue,
		syncStatus:   syncstatus.NewNoOpSyncStatus(),
	}
	return &syncHandlerFixture{
		ctrl:             ctrl,
		objectTreeMock:   objectTreeMock,
		receiveQueueMock: receiveQueue,
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

	t.Run("handle head update message", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := MarshallTreeMessage(treeMsg, "spaceId", treeId, "")

		syncReq := &treechangeproto.TreeSyncMessage{}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.syncProtocolMock.EXPECT().HeadUpdate(ctx, fx.senderId, gomock.Any()).Return(syncReq, nil)
		fx.syncClientMock.EXPECT().QueueRequest(fx.senderId, fx.treeId, syncReq).Return(nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("handle head update message, empty sync request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := MarshallTreeMessage(treeMsg, "spaceId", treeId, "")

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.syncProtocolMock.EXPECT().HeadUpdate(ctx, fx.senderId, gomock.Any()).Return(nil, nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("handle full sync request returns error", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullRequest := &treechangeproto.TreeFullSyncRequest{}
		treeMsg := treechangeproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, _ := MarshallTreeMessage(treeMsg, "spaceId", treeId, "")

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.Equal(t, err, ErrMessageIsRequest)
	})

	t.Run("handle full sync response", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{}
		treeMsg := treechangeproto.WrapFullResponse(fullSyncResponse, chWithId)
		objectMsg, _ := MarshallTreeMessage(treeMsg, "spaceId", treeId, "")

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.syncProtocolMock.EXPECT().FullSyncResponse(ctx, fx.senderId, gomock.Any()).Return(nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
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
		objectMsg, _ := MarshallTreeMessage(treeMsg, "spaceId", treeId, "")

		syncResp := &treechangeproto.TreeSyncMessage{}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.syncProtocolMock.EXPECT().FullSyncRequest(ctx, fx.senderId, gomock.Any()).Return(syncResp, nil)

		res, err := fx.syncHandler.HandleRequest(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("handle request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullRequest := &treechangeproto.TreeFullSyncRequest{}
		treeMsg := treechangeproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, _ := MarshallTreeMessage(treeMsg, "spaceId", treeId, "")

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
			objectMsg, _ := MarshallTreeMessage(msg, "spaceId", treeId, "")

			_, err := fx.syncHandler.HandleRequest(ctx, fx.senderId, objectMsg)
			require.Equal(t, err, ErrMessageIsNotRequest)
		}
	})
}
