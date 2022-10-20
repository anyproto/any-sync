package syncservice

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/mock_syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter/mock_treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree/mock_objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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
	spaceId := "spaceId"
	cacheMock := mock_treegetter.NewMockTreeGetter(ctrl)
	syncClientMock := mock_syncservice.NewMockSyncClient(ctrl)
	objectTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))

	syncHandler := newSyncHandler(spaceId, cacheMock, syncClientMock)
	t.Run("head update non empty all heads added", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &spacesyncproto.ObjectHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		msg := spacesyncproto.WrapHeadUpdate(headUpdate, chWithId, treeId, "")
		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
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

		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("head update non empty heads not added", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &spacesyncproto.ObjectHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		fullRequest := &spacesyncproto.ObjectSyncMessage{}
		msg := spacesyncproto.WrapHeadUpdate(headUpdate, chWithId, treeId, "")
		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
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
			CreateFullSyncRequest(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"}), gomock.Eq("")).
			Return(fullRequest, nil)

		syncClientMock.EXPECT().SendAsync(gomock.Eq([]string{senderId}), gomock.Eq(fullRequest))
		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("head update non empty equal heads", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &spacesyncproto.ObjectHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		msg := spacesyncproto.WrapHeadUpdate(headUpdate, chWithId, treeId, "")
		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"})

		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("head update empty", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &spacesyncproto.ObjectHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}
		fullRequest := &spacesyncproto.ObjectSyncMessage{}
		msg := spacesyncproto.WrapHeadUpdate(headUpdate, chWithId, treeId, "")
		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		syncClientMock.EXPECT().
			CreateFullSyncRequest(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"}), gomock.Eq("")).
			Return(fullRequest, nil)

		syncClientMock.EXPECT().SendAsync(gomock.Eq([]string{senderId}), gomock.Eq(fullRequest))
		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("head update empty equal heads", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &spacesyncproto.ObjectHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}
		msg := spacesyncproto.WrapHeadUpdate(headUpdate, chWithId, treeId, "")
		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)

		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"})

		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})
}

func TestSyncHandler_HandleFullSyncRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	spaceId := "spaceId"
	cacheMock := mock_treegetter.NewMockTreeGetter(ctrl)
	syncClientMock := mock_syncservice.NewMockSyncClient(ctrl)
	objectTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))

	syncHandler := newSyncHandler(spaceId, cacheMock, syncClientMock)
	t.Run("full sync request with change", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		msg := spacesyncproto.WrapFullRequest(&spacesyncproto.ObjectFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}, chWithId, treeId, "")
		fullRequest := &spacesyncproto.ObjectSyncMessage{}

		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
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
			CreateFullSyncResponse(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"}), gomock.Eq("")).
			Return(fullRequest, nil)

		syncClientMock.EXPECT().SendAsync(gomock.Eq([]string{senderId}), gomock.Eq(fullRequest))
		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("full sync request with change same heads", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		msg := spacesyncproto.WrapFullRequest(&spacesyncproto.ObjectFullSyncRequest{
			Heads:        []string{"h2"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h2"},
		}, chWithId, treeId, "")
		fullRequest := &spacesyncproto.ObjectSyncMessage{}

		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h2"}), gomock.Eq([]string{"h2"}), gomock.Eq("")).
			Return(fullRequest, nil)

		syncClientMock.EXPECT().SendAsync(gomock.Eq([]string{senderId}), gomock.Eq(fullRequest))
		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("full sync request without change", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		msg := spacesyncproto.WrapFullRequest(&spacesyncproto.ObjectFullSyncRequest{
			Heads:        []string{"h1"},
			SnapshotPath: []string{"h1"},
		}, chWithId, treeId, "")
		fullRequest := &spacesyncproto.ObjectSyncMessage{}

		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
		syncClientMock.EXPECT().
			CreateFullSyncResponse(gomock.Eq(objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"}), gomock.Eq("")).
			Return(fullRequest, nil)

		syncClientMock.EXPECT().SendAsync(gomock.Eq([]string{senderId}), gomock.Eq(fullRequest))
		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("full sync request with get tree error", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		msg := spacesyncproto.WrapFullRequest(&spacesyncproto.ObjectFullSyncRequest{
			Heads:        []string{"h1"},
			SnapshotPath: []string{"h1"},
		}, chWithId, treeId, "")

		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(nil, fmt.Errorf("some"))

		syncClientMock.EXPECT().
			SendAsync(gomock.Eq([]string{senderId}), gomock.Any())
		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.Error(t, err)
	})
}

func TestSyncHandler_HandleFullSyncResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	spaceId := "spaceId"
	cacheMock := mock_treegetter.NewMockTreeGetter(ctrl)
	syncClientMock := mock_syncservice.NewMockSyncClient(ctrl)
	objectTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))

	syncHandler := newSyncHandler(spaceId, cacheMock, syncClientMock)
	t.Run("full sync response with change", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		msg := spacesyncproto.WrapFullResponse(&spacesyncproto.ObjectFullSyncResponse{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}, chWithId, treeId, "")

		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"})
		objectTreeMock.EXPECT().
			HasChanges(gomock.Eq([]string{"h1"})).
			Return(false)
		objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq([]*treechangeproto.RawTreeChangeWithId{chWithId})).
			Return(tree.AddResult{}, nil)

		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})

	t.Run("full sync response same heads", func(t *testing.T) {
		treeId := "treeId"
		senderId := "senderId"
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		msg := spacesyncproto.WrapFullResponse(&spacesyncproto.ObjectFullSyncResponse{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}, chWithId, treeId, "")

		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, treeId).
			Return(objectTreeMock, nil)
		objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"})

		err := syncHandler.HandleMessage(ctx, senderId, msg)
		require.NoError(t, err)
	})
}
