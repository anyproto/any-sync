package synctree

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/syncstatus/mock_syncstatus"
	"github.com/anyproto/any-sync/net/peer"
)

func TestSyncHandler_Sync(t *testing.T) {
	t.Run("head update ok", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		heads := []string{rawCh.Id}
		changes := []*treechangeproto.RawTreeChangeWithId{rawCh}
		treeHeadUpdate := &treechangeproto.TreeHeadUpdate{
			Changes:      changes,
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
		fx.syncStatus.EXPECT().HeadsReceive("peerId", "objectId", heads)
		fx.tree.EXPECT().AddRawChangesFromPeer(ctx, "peerId", objecttree.RawChangesPayload{
			NewHeads:   heads,
			RawChanges: changes,
		}).Return(objecttree.AddResult{
			OldHeads: []string{"head"},
			Heads:    heads,
			Added:    []*treechangeproto.RawTreeChangeWithId{rawCh},
			Mode:     objecttree.Append,
		}, nil)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.syncStatus, headUpdate)
		require.NoError(t, err)
		require.Nil(t, req)
	})
	t.Run("head update different heads after add", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.finish()
		rawCh := &treechangeproto.RawTreeChangeWithId{
			RawChange: []byte("abcd"),
			Id:        "chId",
		}
		heads := []string{rawCh.Id}
		changes := []*treechangeproto.RawTreeChangeWithId{rawCh}
		treeHeadUpdate := &treechangeproto.TreeHeadUpdate{
			Changes:      changes,
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
		fx.syncStatus.EXPECT().HeadsReceive("peerId", "objectId", heads)
		fx.tree.EXPECT().AddRawChangesFromPeer(ctx, "peerId", objecttree.RawChangesPayload{
			NewHeads:   heads,
			RawChanges: changes,
		}).Return(objecttree.AddResult{
			OldHeads: []string{"head"},
			Heads:    []string{"other"},
			Added:    []*treechangeproto.RawTreeChangeWithId{rawCh},
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
	t.Run("head update no changes, same heads", func(t *testing.T) {
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
	t.Run("head update no changes, different heads", func(t *testing.T) {
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
