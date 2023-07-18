package synctree

import (
	"context"
	"fmt"
	"testing"

	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/peermanager/mock_peermanager"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peer/mock_peer"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
		resp, err := fx.treeGetter.treeRequestLoop(ctx)
		require.NoError(t, err)
		require.Equal(t, "id", resp.RootChange.Id)
	})

	t.Run("request fails", func(t *testing.T) {
		fx := newTreeRemoteGetterFixture(t)
		defer fx.stop()
		treeRequest := &treechangeproto.TreeSyncMessage{}
		mockPeer := mock_peer.NewMockPeer(fx.ctrl)
		mockPeer.EXPECT().Id().AnyTimes().Return(peerId)
		fx.peerGetterMock.EXPECT().GetResponsiblePeers(ctx).Return([]peer.Peer{mockPeer}, nil)
		fx.syncClientMock.EXPECT().CreateNewTreeRequest().Return(treeRequest)
		fx.syncClientMock.EXPECT().SendRequest(ctx, peerId, fx.treeGetter.treeId, treeRequest).AnyTimes().Return(nil, fmt.Errorf("some"))
		_, err := fx.treeGetter.treeRequestLoop(ctx)
		require.Error(t, err)
	})
}
