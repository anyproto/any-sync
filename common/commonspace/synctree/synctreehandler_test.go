package synctree

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/mock_synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree/mock_objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sync"
	"testing"
)

type testObjTreeMock struct {
	*mock_tree.MockObjectTree
	m sync.Mutex
}

func newTestObjMock(mockTree *mock_tree.MockObjectTree) *testObjTreeMock {
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

func TestSyncHandler_HandleHeadUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	objectTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))

	syncHandler := newSyncTreeHandler(objectTreeMock, syncClientMock)
	log = zap.NewNop().Sugar()

	t.Run("head update non empty all heads added", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")

		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq([]*treechangeproto.RawTreeChangeWithId{chWithId})).
			Return(tree.AddResult{}, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2", "h1"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(true)

		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update non empty heads not added", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")
		fullRequest := &treechangeproto.TreeSyncMessage{}

		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq([]*treechangeproto.RawTreeChangeWithId{chWithId})).
			Return(tree.AddResult{}, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		syncClientMock.EXPECT().
			CreateFullSyncRequest(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullRequest, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})

		syncClientMock.EXPECT().SendAsync(gomock.Eq(senderId), gomock.Eq(fullRequest), gomock.Eq(""))

		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update non empty equal heads", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")

		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"})

		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update empty", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")
		fullRequest := &treechangeproto.TreeSyncMessage{}

		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		syncClientMock.EXPECT().
			CreateFullSyncRequest(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullRequest, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})

		syncClientMock.EXPECT().SendAsync(gomock.Eq(senderId), gomock.Eq(fullRequest), gomock.Eq(""))

		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("head update empty equal heads", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")

		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"})

		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})
}

func TestSyncHandler_HandleFullSyncRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	objectTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))

	syncHandler := newSyncTreeHandler(objectTreeMock, syncClientMock)
	log = zap.NewNop().Sugar()
	t.Run("full sync request with change", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")
		fullResponse := &treechangeproto.TreeSyncMessage{}
		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().Header().Return(nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq([]*treechangeproto.RawTreeChangeWithId{chWithId})).
			Return(tree.AddResult{}, nil)
		syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)
		syncClientMock.EXPECT().SendAsync(gomock.Eq(senderId), gomock.Eq(fullResponse), gomock.Eq(""))
		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync request with change same heads", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")
		fullResponse := &treechangeproto.TreeSyncMessage{}
		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().Header().Return(nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"})
		syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)
		syncClientMock.EXPECT().SendAsync(gomock.Eq(senderId), gomock.Eq(fullResponse), gomock.Eq(""))
		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync request without change but with reply id", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		replyId := "replyId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, replyId)
		fullResponse := &treechangeproto.TreeSyncMessage{}
		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().Header().Return(nil)
		syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)
		syncClientMock.EXPECT().SendAsync(gomock.Eq(senderId), gomock.Eq(fullResponse), gomock.Eq(replyId))
		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync request with add raw changes error", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		treeMsg := treechangeproto.WrapFullRequest(fullSyncRequest, chWithId)
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, "")
		objectTreeMock.EXPECT().
			ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().Header().Return(nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq([]*treechangeproto.RawTreeChangeWithId{chWithId})).
			Return(tree.AddResult{}, fmt.Errorf(""))
		syncClientMock.EXPECT().SendAsync(gomock.Eq(senderId), gomock.Any(), gomock.Eq(""))
		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.Error(t, err)
	})
}

func TestSyncHandler_HandleFullSyncResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	objectTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))

	syncHandler := newSyncTreeHandler(objectTreeMock, syncClientMock)
	log = zap.NewNop().Sugar()

	t.Run("full sync response with change", func(t *testing.T) {
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
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, replyId)
		objectTreeMock.EXPECT().ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq([]*treechangeproto.RawTreeChangeWithId{chWithId})).
			Return(tree.AddResult{}, nil)

		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})

	t.Run("full sync response with same heads", func(t *testing.T) {
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
		objectMsg, _ := marshallTreeMessage(treeMsg, treeId, replyId)
		objectTreeMock.EXPECT().ID().AnyTimes().Return(treeId)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"})

		err := syncHandler.HandleMessage(ctx, senderId, objectMsg)
		require.NoError(t, err)
	})
}
