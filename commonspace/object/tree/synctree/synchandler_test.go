package synctree

import (
	"errors"
	"google.golang.org/protobuf/proto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response/mock_response"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
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
		marshaled, err := wrapped.MarshalVT()
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
		marshaled, err := wrapped.MarshalVT()
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
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
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
		marshaled, err := wrapped.MarshalVT()
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
		marshaled, err := wrapped.MarshalVT()
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
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
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
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		createResponseProducer = func(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (response.ResponseProducer, error) {
			return producer, nil
		}
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		storage := mock_objecttree.NewMockStorage(fx.ctrl)
		fx.tree.EXPECT().HasChanges("peerHead").Return(false)
		fx.tree.EXPECT().Root().Return(&objecttree.Change{Id: "snapshotId"})
		fx.tree.EXPECT().Id().Return("objectId")
		fx.tree.EXPECT().Storage().Return(storage)
		storage.EXPECT().Has(gomock.Any(), "peerHead").Return(false, nil)
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
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
	t.Run("heads are different, we have all their heads, we don't send request", func(t *testing.T) {
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
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		setResponseProducer(t, producer, heads, fullRequest.SnapshotPath)
		fx.tree.EXPECT().HasChanges("peerHead").Return(true)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		resp := &response.Response{
			Heads:    []string{"curHead"},
			ObjectId: "objectId",
			Changes:  []*treechangeproto.RawTreeChangeWithId{rawCh},
		}
		producer.EXPECT().NewResponse(gomock.Any()).Return(resp, nil)
		producer.EXPECT().NewResponse(gomock.Any()).Return(&response.Response{}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		var sent []*treechangeproto.TreeFullSyncResponse
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			sent = append(sent, decodeFullSyncResponse(t, resp))
			return nil
		})
		require.NoError(t, err)
		require.Nil(t, req)
		require.Len(t, sent, 1)
		assert.Equal(t, []string{"curHead"}, sent[0].Heads)
		require.Len(t, sent[0].Changes, 1)
		assert.Equal(t, "chId", sent[0].Changes[0].Id)
		assert.Equal(t, []byte("abcd"), sent[0].Changes[0].RawChange)
	})
	t.Run("heads are different, we store their heads below our root snapshot, we don't send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"inTreeHead", "belowRootHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        heads,
			SnapshotPath: []string{"root"},
		}
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		setResponseProducer(t, producer, heads, fullRequest.SnapshotPath)
		storage := mock_objecttree.NewMockStorage(fx.ctrl)
		fx.tree.EXPECT().HasChanges("inTreeHead").Return(true)
		fx.tree.EXPECT().HasChanges("belowRootHead").Return(false)
		fx.tree.EXPECT().Root().Return(&objecttree.Change{Id: "snapshotId"})
		fx.tree.EXPECT().Id().Return("objectId")
		fx.tree.EXPECT().Storage().Return(storage)
		storage.EXPECT().Has(gomock.Any(), "belowRootHead").Return(true, nil)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		resp := &response.Response{
			Heads:    []string{"curHead"},
			ObjectId: "objectId",
			Changes:  []*treechangeproto.RawTreeChangeWithId{rawCh},
		}
		producer.EXPECT().NewResponse(gomock.Any()).Return(resp, nil)
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
		require.Equal(t, 1, callCount)
	})
	t.Run("heads are different, storage lookup fails, we send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"peerHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        heads,
			SnapshotPath: []string{"root"},
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		setResponseProducer(t, producer, heads, fullRequest.SnapshotPath)
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		storage := mock_objecttree.NewMockStorage(fx.ctrl)
		fx.tree.EXPECT().HasChanges("peerHead").Return(false)
		fx.tree.EXPECT().Root().Return(&objecttree.Change{Id: "snapshotId"})
		fx.tree.EXPECT().Id().Return("objectId")
		fx.tree.EXPECT().Storage().Return(storage)
		storage.EXPECT().Has(gomock.Any(), "peerHead").Return(false, errors.New("storage failed"))
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		producer.EXPECT().NewResponse(gomock.Any()).Return(&response.Response{}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
	})
	t.Run("heads are different, tree loaded from its root, their head only in storage, we send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"storedHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        heads,
			SnapshotPath: []string{"objectId"},
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		setResponseProducer(t, producer, heads, fullRequest.SnapshotPath)
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		// the store holds storedHead, but without its parent, so it is not attached
		storage := mock_objecttree.NewMockStorage(fx.ctrl)
		fx.tree.EXPECT().HasChanges("storedHead").Return(false)
		fx.tree.EXPECT().Root().Return(&objecttree.Change{Id: "objectId"})
		fx.tree.EXPECT().Id().Return("objectId")
		fx.tree.EXPECT().Storage().Return(storage).AnyTimes()
		storage.EXPECT().Has(gomock.Any(), "storedHead").Return(true, nil).AnyTimes()
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		producer.EXPECT().NewResponse(gomock.Any()).Return(&response.Response{}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
	})
	t.Run("heads partly different, they have a head we lack, we send our head and request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"headA", "headC"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        heads,
			SnapshotPath: []string{"objectId"},
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		setResponseProducer(t, producer, heads, fullRequest.SnapshotPath)
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		fx.tree.EXPECT().HasChanges("headA").Return(true).AnyTimes()
		fx.tree.EXPECT().HasChanges("headC").Return(false)
		fx.tree.EXPECT().Root().Return(&objecttree.Change{Id: "objectId"})
		fx.tree.EXPECT().Id().Return("objectId")
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
		fx.tree.EXPECT().Heads().Return([]string{"headA", "headB"})
		rawB := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("changeB"),
			Id:        "headB",
		}
		producer.EXPECT().NewResponse(gomock.Any()).Return(&response.Response{
			Heads:    []string{"headA", "headB"},
			ObjectId: "objectId",
			Changes:  []*treechangeproto.RawTreeChangeWithId{rawB},
		}, nil)
		producer.EXPECT().NewResponse(gomock.Any()).Return(&response.Response{}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		var sent []*treechangeproto.TreeFullSyncResponse
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			sent = append(sent, decodeFullSyncResponse(t, resp))
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
		require.Len(t, sent, 1)
		assert.Equal(t, []string{"headA", "headB"}, sent[0].Heads)
		require.Len(t, sent[0].Changes, 1)
		assert.Equal(t, "headB", sent[0].Changes[0].Id)
		assert.Equal(t, []byte("changeB"), sent[0].Changes[0].RawChange)
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
		marshaled, err := wrapped.MarshalVT()
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
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		createResponseProducer = func(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (response.ResponseProducer, error) {
			return producer, nil
		}
		fx.tree.EXPECT().HasChanges(gomock.InAnyOrder(heads)).Return(false)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		resp := &response.Response{
			Heads: heads,
		}
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
		producer.EXPECT().EmptyResponse().Return(resp, nil)
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
	t.Run("they have our heads and a head in our tree, we don't send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"treeHead", "curHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        heads,
			SnapshotPath: []string{"root"},
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		setResponseProducer(t, producer, heads, fullRequest.SnapshotPath)
		fx.tree.EXPECT().HasChanges(gomock.InAnyOrder(heads)).Return(true)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		producer.EXPECT().EmptyResponse().Return(&response.Response{
			Heads:    []string{"curHead"},
			ObjectId: "objectId",
		}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		var sent []*treechangeproto.TreeFullSyncResponse
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			sent = append(sent, decodeFullSyncResponse(t, resp))
			return nil
		})
		require.NoError(t, err)
		require.Nil(t, req)
		require.Len(t, sent, 1)
		assert.Equal(t, []string{"curHead"}, sent[0].Heads)
		assert.Empty(t, sent[0].Changes)
	})
	t.Run("they have our heads and a head only in our storage, we send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"storedHead", "curHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        heads,
			SnapshotPath: []string{"root"},
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		producer := mock_response.NewMockResponseProducer(fx.ctrl)
		setResponseProducer(t, producer, heads, fullRequest.SnapshotPath)
		// no Storage() expectation: a stored head may be an orphan whose parent we lack
		fx.tree.EXPECT().HasChanges(gomock.InAnyOrder(heads)).Return(false)
		fx.tree.EXPECT().Heads().Return([]string{"curHead"})
		returnReq := &objectmessages.Request{
			Bytes: []byte("abcde"),
		}
		fx.client.EXPECT().CreateFullSyncRequest("peerId", fx.tree).Return(returnReq, nil)
		producer.EXPECT().EmptyResponse().Return(&response.Response{
			Heads:    []string{"curHead"},
			ObjectId: "objectId",
		}, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, returnReq, req)
	})
	t.Run("we have exactly same heads, we don't send request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		heads := []string{"curHead"}
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Heads: heads,
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
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
		producer.EXPECT().EmptyResponse().Return(resp, nil)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		callCount := 0
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			require.NotNil(t, resp)
			callCount++
			return nil
		})
		require.NoError(t, err)
		require.Nil(t, req)
		require.Equal(t, 1, callCount)
	})
}

// setResponseProducer makes the handler use producer and checks that it is built
// for the requester's heads and snapshot path.
func setResponseProducer(t *testing.T, producer response.ResponseProducer, wantHeads, wantSnapshotPath []string) {
	createResponseProducer = func(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (response.ResponseProducer, error) {
		assert.Equal(t, wantHeads, theirHeads)
		assert.Equal(t, wantSnapshotPath, theirSnapshotPath)
		return producer, nil
	}
}

func decodeFullSyncResponse(t *testing.T, resp proto.Message) *treechangeproto.TreeFullSyncResponse {
	t.Helper()
	msg, ok := resp.(*spacesyncproto.ObjectSyncMessage)
	require.True(t, ok)
	treeMsg := &treechangeproto.TreeSyncMessage{}
	require.NoError(t, treeMsg.UnmarshalVT(msg.Payload))
	fullResp := treeMsg.GetContent().GetFullSyncResponse()
	require.NotNil(t, fullResp)
	return fullResp
}

func TestSyncHandler_HandleStreamRequest_Probe(t *testing.T) {
	t.Run("probe request, we send root and heads without changes, no request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		fullRequest := &treechangeproto.TreeFullSyncRequest{
			Probe: true,
		}
		wrapped := treechangeproto.WrapFullRequest(fullRequest, nil)
		marshaled, err := wrapped.MarshalVT()
		require.NoError(t, err)
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		root := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("rootChange"),
			Id:        "objectId",
		}
		fx.tree.EXPECT().Heads().AnyTimes().Return([]string{"curHead"})
		fx.tree.EXPECT().SnapshotPath().Return([]string{"objectId"}, nil)
		fx.tree.EXPECT().Header().Return(root)
		ctx = peer.CtxWithPeerId(ctx, "peerId")
		callCount := 0
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testUpdater{}, func(resp proto.Message) error {
			callCount++
			msg, ok := resp.(*spacesyncproto.ObjectSyncMessage)
			require.True(t, ok)
			treeMsg := &treechangeproto.TreeSyncMessage{}
			require.NoError(t, treeMsg.UnmarshalVT(msg.Payload))
			fullResp := treeMsg.GetContent().GetFullSyncResponse()
			require.NotNil(t, fullResp)
			require.Equal(t, []string{"curHead"}, fullResp.Heads)
			require.Empty(t, fullResp.Changes)
			require.Equal(t, root.Id, treeMsg.RootChange.Id)
			require.Equal(t, root.RawChange, treeMsg.RootChange.RawChange)
			return nil
		})
		require.NoError(t, err)
		require.Nil(t, req)
		require.Equal(t, 1, callCount)
	})
}

func TestNewProbeRequest(t *testing.T) {
	req := NewProbeRequest("peerId", "spaceId", "objectId")
	protoMsg, err := req.Proto()
	require.NoError(t, err)
	msg, ok := protoMsg.(*spacesyncproto.ObjectSyncMessage)
	require.True(t, ok)
	treeMsg := &treechangeproto.TreeSyncMessage{}
	require.NoError(t, treeMsg.UnmarshalVT(msg.Payload))
	fullReq := treeMsg.GetContent().GetFullSyncRequest()
	require.NotNil(t, fullReq)
	require.True(t, fullReq.Probe)
	require.Empty(t, fullReq.Heads)
	require.Empty(t, fullReq.Changes)
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
