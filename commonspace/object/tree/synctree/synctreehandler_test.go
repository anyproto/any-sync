package synctree

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sync"
	"testing"
)

type testObjTreeMock struct {
	*mock_objecttree.MockObjectTree
	m sync.Mutex
}

func newTestObjMock(mockTree *mock_objecttree.MockObjectTree) *testObjTreeMock {
	return &testObjTreeMock{
		MockObjectTree: mockTree,
	}
}

func (t *testObjTreeMock) Lock() {
	t.m.Lock()
}

func (t *testObjTreeMock) Unlock() {
	t.m.Unlock()
}

type syncHandlerFixture struct {
	ctrl             *gomock.Controller
	syncClientMock   *mock_synctree.MockSyncClient
	objectTreeMock   *testObjTreeMock
	receiveQueueMock ReceiveQueue

	syncHandler *syncTreeHandler
}

func newSyncHandlerFixture(t *testing.T) *syncHandlerFixture {
	ctrl := gomock.NewController(t)
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	objectTreeMock := newTestObjMock(mock_objecttree.NewMockObjectTree(ctrl))
	receiveQueue := newReceiveQueue(5)

	syncHandler := &syncTreeHandler{
		objTree:    objectTreeMock,
		syncClient: syncClientMock,
		queue:      receiveQueue,
		syncStatus: syncstatus.NewNoOpSyncStatus(),
	}
	return &syncHandlerFixture{
		ctrl:             ctrl,
		syncClientMock:   syncClientMock,
		objectTreeMock:   objectTreeMock,
		receiveQueueMock: receiveQueue,
		syncHandler:      syncHandler,
	}
}

func (fx *syncHandlerFixture) stop() {
	fx.ctrl.Finish()
}

func TestSyncHandler_HandleHeadUpdate(t *testing.T) {
	ctx := context.Background()
	log = zap.NewNop().Sugar()

	t.Run("head update non empty all heads added", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).Times(2)
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).Return(false)
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2", "h1"})
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).Return(true)

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update non empty heads not added", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")
		fullRequest := &treechangeproto.TreeSyncMessage{}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).AnyTimes()
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).Return(false)
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).Return(false)
		fx.syncClientMock.EXPECT().
			CreateFullSyncRequest(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullRequest, nil)
		fx.syncClientMock.EXPECT().SendWithReply(gomock.Any(), gomock.Eq(senderId), gomock.Eq(fullRequest), gomock.Eq(""))

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update non empty equal heads", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h1"}).AnyTimes()

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update empty", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")
		fullRequest := &treechangeproto.TreeSyncMessage{}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).AnyTimes()
		fx.syncClientMock.EXPECT().
			CreateFullSyncRequest(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullRequest, nil)
		fx.syncClientMock.EXPECT().SendWithReply(gomock.Any(), gomock.Eq(senderId), gomock.Eq(fullRequest), gomock.Eq(""))

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update empty equal heads", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h1"}).AnyTimes()

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})
}

func TestSyncHandler_HandleFullSyncRequest(t *testing.T) {
	ctx := context.Background()
	log = zap.NewNop().Sugar()

	t.Run("full sync request with change", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")
		fullResponse := &treechangeproto.TreeSyncMessage{}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Header().Return(nil)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).AnyTimes()
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).Return(false)
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)
		fx.syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)
		fx.syncClientMock.EXPECT().SendWithReply(gomock.Any(), gomock.Eq(senderId), gomock.Eq(fullResponse), gomock.Eq(""))

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync request with change same heads", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")
		fullResponse := &treechangeproto.TreeSyncMessage{}

		fx.objectTreeMock.EXPECT().
			Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Header().Return(nil)
		fx.objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"}).AnyTimes()
		fx.syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)
		fx.syncClientMock.EXPECT().SendWithReply(gomock.Any(), gomock.Eq(senderId), gomock.Eq(fullResponse), gomock.Eq(""))

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync request without change but with reply id", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		replyId := "replyId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, replyId)
		fullResponse := &treechangeproto.TreeSyncMessage{}

		fx.objectTreeMock.EXPECT().
			Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Header().Return(nil)
		fx.syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)
		fx.syncClientMock.EXPECT().SendWithReply(gomock.Any(), gomock.Eq(senderId), gomock.Eq(fullResponse), gomock.Eq(replyId))

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync request with add raw changes error", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, "")

		fx.objectTreeMock.EXPECT().
			Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().Header().Return(nil)
		fx.objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		fx.objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, fmt.Errorf(""))
		fx.syncClientMock.EXPECT().SendWithReply(gomock.Any(), gomock.Eq(senderId), gomock.Any(), gomock.Eq(""))

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.Error(t, err)
	})
}

func TestSyncHandler_HandleFullSyncResponse(t *testing.T) {
	ctx := context.Background()
	log = zap.NewNop().Sugar()

	t.Run("full sync response with change", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		replyId := "replyId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullResponse(fullSyncResponse, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, replyId)

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"}).AnyTimes()
		fx.objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync response with same heads", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		treeId := "treeId"
		senderId := "senderId"
		replyId := "replyId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullResponse(fullSyncResponse, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, "spaceId", treeId, replyId)

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(treeId)
		fx.objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"}).AnyTimes()

		err := fx.syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})
}
