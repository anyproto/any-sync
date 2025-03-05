package synctree

import (
	"testing"

	"github.com/anyproto/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response/mock_response"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/syncstatus/mock_syncstatus"
	"github.com/anyproto/any-sync/net/peer"
)

type testUpdater struct {
}

func (t testUpdater) UpdateQueueSize(size uint64, msgType int, add bool) {
}

func TestSyncHandler_HeadUpdate(t *testing.T) {
	t.Run("head update ok, everything added, we don't send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		rawCh := objecttree.StorageChange{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		heads := []string{rawCh.Id}
		changes := []*treechangeproto.RawTreeChangeWithId{rawCh.RawTreeChangeWithId()}
		treeHeadUpdate := &treechangeproto.TreeHeadUpdate{
			Changes:      changes,
			Heads:        heads,
			SnapshotPath: []string{rawCh.Id},
		}
		wrapped := treechangeproto.WrapHeadUpdate(treeHeadUpdate, rawCh.RawTreeChangeWithId())
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		headUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		fx.syncStatus.EXPECT().HeadsReceive("peerId", "objectId", heads)
		fx.tree.EXPECT().Heads().AnyTimes().Return([]string{rawCh.Id})
		fx.tree.EXPECT().AddRawChangesFromPeer(ctx, "peerId", objecttree.RawChangesPayload{
			NewHeads:     heads,
			RawChanges:   changes,
			SnapshotPath: []string{rawCh.Id},
		}).Return(objecttree.AddResult{
			OldHeads: []string{"head"},
			Heads:    heads,
			Added:    []objecttree.StorageChange{rawCh},
			Mode:     objecttree.Append,
		}, nil)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.syncStatus, headUpdate)
		require.NoError(t, err)
		require.Nil(t, req)
	})
	t.Run("head update different heads after add, we send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		rawCh := objecttree.StorageChange{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		heads := []string{rawCh.Id}
		changes := []*treechangeproto.RawTreeChangeWithId{rawCh.RawTreeChangeWithId()}
		treeHeadUpdate := &treechangeproto.TreeHeadUpdate{
			Changes:      changes,
			Heads:        heads,
			SnapshotPath: []string{rawCh.Id},
		}
		wrapped := treechangeproto.WrapHeadUpdate(treeHeadUpdate, rawCh.RawTreeChangeWithId())
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		headUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		fx.syncStatus.EXPECT().HeadsReceive("peerId", "objectId", heads)
		fx.tree.EXPECT().Heads().AnyTimes().Return([]string{rawCh.Id})
		fx.tree.EXPECT().AddRawChangesFromPeer(ctx, "peerId", objecttree.RawChangesPayload{
			NewHeads:     heads,
			RawChanges:   changes,
			SnapshotPath: []string{rawCh.Id},
		}).Return(objecttree.AddResult{
			OldHeads: []string{"head"},
			Heads:    []string{"other"},
			Added:    []objecttree.StorageChange{rawCh},
			Mode:     objecttree.Append,
		}, nil)
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcd"),
		}
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.syncStatus, headUpdate)
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
	})
	t.Run("head update no changes, same heads, we don't send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		heads := []string{rawCh.Id}
		treeHeadUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        heads,
			SnapshotPath: []string{rawCh.Id},
		}
		wrapped := treechangeproto.WrapHeadUpdate(treeHeadUpdate, rawCh)
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		headUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		fx.tree.EXPECT().Heads().AnyTimes().Return([]string{rawCh.Id})
		fx.syncStatus.EXPECT().HeadsReceive("peerId", "objectId", heads)
		fx.syncStatus.EXPECT().HeadsApply("peerId", "objectId", heads, true)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.syncStatus, headUpdate)
		require.NoError(t, err)
		require.Nil(t, req)
	})
	t.Run("head update no changes, different heads, we send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		heads := []string{rawCh.Id}
		treeHeadUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        heads,
			SnapshotPath: []string{rawCh.Id},
		}
		wrapped := treechangeproto.WrapHeadUpdate(treeHeadUpdate, rawCh)
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		headUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		fx.tree.EXPECT().Heads().AnyTimes().Return([]string{"otherIds"})
		fx.tree.EXPECT().HasChanges(gomock.Any()).Return(false)
		fx.syncStatus.EXPECT().HeadsReceive("peerId", "objectId", heads)
		fx.syncStatus.EXPECT().HeadsApply("peerId", "objectId", heads, false)
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcd"),
		}
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.syncStatus, headUpdate)
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
	})
}

func TestSyncHandler_HandleStreamRequest(t *testing.T) {
	t.Run("heads are different, we send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"peerHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        heads,
			SnapshotPath: []string{"root"},
		}
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		createResponseProducer = func(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (response.ResponseProducer, error) {
			return producer, nil
		}
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		resp := &response.Response{
			Heads:    heads,
			ObjectId: "objectId",
			Changes: []*treechangeproto.RawTreeChangeWithId{
				rawCh,
			},
		}
		producer.EXPECT().NewResponse(gomock.Any()).Times(2).Return(resp, nil)
		producer.EXPECT().NewResponse(gomock.Any()).Return(&response.Response{}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		callCount := 0
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			require.NotNil(t, resp)
			callCount++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
		require.Equal(t, 2, callCount)
	})
	t.Run("request has no heads = peer doesn't have tree, we don't send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"peerHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{}
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		createResponseProducer = func(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (response.ResponseProducer, error) {
			return producer, nil
		}
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		resp := &response.Response{
			Heads:    heads,
			ObjectId: "objectId",
			Changes: []*treechangeproto.RawTreeChangeWithId{
				rawCh,
			},
		}
		producer.EXPECT().NewResponse(gomock.Any()).Times(2).Return(resp, nil)
		producer.EXPECT().NewResponse(gomock.Any()).Return(&response.Response{}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		callCount := 0
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			require.NotNil(t, resp)
			callCount++
			return nil
		})
		require.NoError(t, err)
		require.Nil(t, req)
		require.Equal(t, 2, callCount)
	})
	t.Run("they have our heads and more, we send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"peerHead", "curHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads: heads,
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		createResponseProducer = func(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (response.ResponseProducer, error) {
			return producer, nil
		}
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		resp := &response.Response{
			Heads: heads,
		}
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq)
		producer.EXPECT().EmptyResponse().Return(resp)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		callCount := 0
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			require.NotNil(t, resp)
			callCount++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
		require.Equal(t, 1, callCount)
	})
	t.Run("we have exactly same heads, we don't send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"peerHead", "curHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads: heads,
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.Marshal()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		createResponseProducer = func(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (response.ResponseProducer, error) {
			return producer, nil
		}
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		resp := &response.Response{
			Heads: heads,
		}
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq)
		producer.EXPECT().EmptyResponse().Return(resp)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		callCount := 0
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			require.NotNil(t, resp)
			callCount++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
		require.Equal(t, 1, callCount)
	})
}

func TestSyncHandler_HandleResponse(t *testing.T) {
	t.Run("handle response with changes", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		resp := &response.Response{
			Heads: []string{"abcd"},
			Changes: []*treechangeproto.RawTreeChangeWithId{
				rawCh,
			},
		}
		payload := objecttree.RawChangesPayload{
			NewHeads:   resp.Heads,
			RawChanges: resp.Changes,
		}
		fx.tree.EXPECT().AddRawChangesFromPeer(ctx, "peerId", payload).Return(objecttree.AddResult{}, nil)
		err := fx.syncHandler.HandleResponse(ctx, "peerId", "objectId", resp)
		require.NoError(t, err)
	})
}

type syncHandlerFixture struct {
	ctrl        *gomock.Controller
	tree        *testSyncTreeMock
	client      *mock_synctree.MockSyncClient
	syncStatus  *mock_syncstatus.MockStatusUpdater
	syncHandler *syncHandler
}

type testSyncTreeMock struct {
	*mock_synctree.MockSyncTree
}

func newTestSyncTreeMock(obj *mock_synctree.MockSyncTree) *testSyncTreeMock {
	return &testSyncTreeMock{obj}
}

func (t *testSyncTreeMock) Lock() {
}

func (t *testSyncTreeMock) Unlock() {
}

func newSyncHandlerFixture(t *testing.T) *syncHandlerFixture {
	ctrl := gomock.NewController(t)
	tree := newTestSyncTreeMock(mock_synctree.NewMockSyncTree(ctrl))
	client := mock_synctree.NewMockSyncClient(ctrl)
	syncStatus := mock_syncstatus.NewMockStatusUpdater(ctrl)
	syncHandler := &syncHandler{
		tree:       tree,
		syncClient: client,
		spaceId:    "spaceId",
	}
	return &syncHandlerFixture{
		ctrl:        ctrl,
		tree:        tree,
		client:      client,
		syncStatus:  syncStatus,
		syncHandler: syncHandler,
	}
}

func (fx *syncHandlerFixture) finish() {
	fx.ctrl.Finish()
}
