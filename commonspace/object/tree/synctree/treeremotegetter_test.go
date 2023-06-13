package synctree

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/peermanager/mock_peermanager"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peer/mock_peer"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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
	newRequestTimeout = 20 * time.Millisecond
	retryTimeout := 2 * newRequestTimeout
	ctx := context.Background()
	peerId := "peerId"
	treeRequest := &treechangeproto.TreeSyncMessage{}
	treeResponse := &treechangeproto.TreeSyncMessage{
		RootChange: &treechangeproto.RawTreeChangeWithId{Id: "id"},
	}
	marshalled, _ := proto.Marshal(treeResponse)
	objectResponse := &spacesyncproto.ObjectSyncMessage{
		Payload: marshalled,
	}

	t.Run("request works", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).Return(objectResponse, nil)
		resp, err := fx.treeGetter.treeRequestLoop(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "id", resp.RootChange.Id)
	})

	t.Run("request peerId from context", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		ctx := peer.CtxWithPeerId(ctx, peerId)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).Return(objectResponse, nil)
		resp, err := fx.treeGetter.treeRequestLoop(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "id", resp.RootChange.Id)
	})

	t.Run("request fails", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).Return(nil, fmt.Errorf("failed"))
		_, err := fx.treeGetter.treeRequestLoop(ctx, 0)
		require.Error(t, err)
		require.NotEqual(t, ErrRetryTimeout, err)
	})

	t.Run("retry request success", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).AnyTimes().Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().AnyTimes().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).Times(1).Return(nil, fmt.Errorf("some"))
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).Times(1).Return(objectResponse, nil)
		resp, err := fx.treeGetter.treeRequestLoop(ctx, retryTimeout)
		require.NoError(t, err)
		require.Equal(t, "id", resp.RootChange.Id)
	})

	t.Run("no retry request if error get tree", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).AnyTimes().Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().AnyTimes().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).Times(1).Return(nil, treechangeproto.ErrGetTree)
		_, err := fx.treeGetter.treeRequestLoop(ctx, retryTimeout)
		require.Equal(t, treechangeproto.ErrGetTree, err)
	})

	t.Run("retry get peers success", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).Times(1).Return([]peer.Peer{}, nil)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).Times(1).Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().AnyTimes().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).Times(1).Return(objectResponse, nil)
		resp, err := fx.treeGetter.treeRequestLoop(ctx, retryTimeout)
		require.NoError(t, err)
		require.Equal(t, "id", resp.RootChange.Id)
	})

	t.Run("retry request fail", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		treeRequest := &treechangeproto.TreeSyncMessage{}
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).AnyTimes().Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().AnyTimes().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).AnyTimes().Return(nil, fmt.Errorf("some"))
		_, err := fx.treeGetter.treeRequestLoop(ctx, retryTimeout)
		require.Equal(t, ErrRetryTimeout, err)
	})
}
