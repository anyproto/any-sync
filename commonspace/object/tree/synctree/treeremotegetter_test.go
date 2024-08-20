package synctree

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/peermanager/mock_peermanager"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peer/mock_peer"
)

type treeRemoteGetterFixture struct {
	ctrl *gomock.Controller

	treeGetter     treeRemoteGetter
	syncClientMock *mock_synctree.MockSyncClient
	peerGetterMock *mock_peermanager.MockPeerManager
}

func newTreeRemoteGetterFixture(t *testing.T) *treeRemoteGetterFixture {
	ctrl := gomock.NewController(t)
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	peerGetterMock := mock_peermanager.NewMockPeerManager(ctrl)
	treeGetter := treeRemoteGetter{
		deps: BuildDeps{
			SyncClient: syncClientMock,
			PeerGetter: peerGetterMock,
		},
		treeId: "treeId",
	}
	return &treeRemoteGetterFixture{
		ctrl:           ctrl,
		treeGetter:     treeGetter,
		syncClientMock: syncClientMock,
		peerGetterMock: peerGetterMock,
	}
}

func (fx *treeRemoteGetterFixture) stop() {
	fx.ctrl.Finish()
}

func TestTreeRemoteGetter(t *testing.T) {
	ctx := context.Background()
	peerId := "peerId"
	treeRequest := &objectmessages.Request{}

	t.Run("responsible peers", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		coll := newFullResponseCollector()
		createCollector = func() *fullResponseCollector {
			return coll
		}
		tCtx := peer.CtxWithPeerId(ctx, "*")
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(tCtx).Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest("peerId", "treeId").Return(treeRequest)
		fx.syncClientMock.EXPECT().SendTreeRequest(tCtx, treeRequest, coll).Return(nil)
		retColl, pId, err := fx.treeGetter.treeRequestLoop(tCtx)
		require.NoError(t, err)
		require.Equal(t, "peerId", pId)
		require.Equal(t, coll, retColl)
	})

	t.Run("request fails", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		coll := newFullResponseCollector()
		createCollector = func() *fullResponseCollector {
			return coll
		}
		tCtx := peer.CtxWithPeerId(ctx, "*")
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(tCtx).Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest("peerId", "treeId").Return(treeRequest)
		fx.syncClientMock.EXPECT().SendTreeRequest(tCtx, treeRequest, coll).Return(fmt.Errorf("error"))
		_, _, err := fx.treeGetter.treeRequestLoop(tCtx)
		require.Error(t, err)
	})

	t.Run("no responsible peers", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		coll := newFullResponseCollector()
		createCollector = func() *fullResponseCollector {
			return coll
		}
		tCtx := peer.CtxWithPeerId(ctx, "*")
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(tCtx).Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest("peerId", "treeId").Return(treeRequest)
		fx.syncClientMock.EXPECT().SendTreeRequest(tCtx, treeRequest, coll).Return(fmt.Errorf("error"))
		_, _, err := fx.treeGetter.treeRequestLoop(tCtx)
		require.Error(t, err)
	})
}
