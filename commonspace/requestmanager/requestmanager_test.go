package requestmanager

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/objectsync/mock_objectsync"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto/mock_spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peer/mock_peer"
	"github.com/anyproto/any-sync/net/pool/mock_pool"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"testing"
)

type fixture struct {
	requestManager     *requestManager
	messageHandlerMock *mock_objectsync.MockObjectSync
	peerPoolMock       *mock_pool.MockPool
	clientMock         *mock_spacesyncproto.MockDRPCSpaceSyncClient
	ctrl               *gomock.Controller
}

func newFixture(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	manager := New().(*requestManager)
	peerPoolMock := mock_pool.NewMockPool(ctrl)
	messageHandlerMock := mock_objectsync.NewMockObjectSync(ctrl)
	clientMock := mock_spacesyncproto.NewMockDRPCSpaceSyncClient(ctrl)
	manager.peerPool = peerPoolMock
	manager.handler = messageHandlerMock
	manager.clientFactory = spacesyncproto.ClientFactoryFunc(func(cc drpc.Conn) spacesyncproto.DRPCSpaceSyncClient {
		return clientMock
	})
	manager.ctx, manager.cancel = context.WithCancel(context.Background())
	return &fixture{
		requestManager:     manager,
		messageHandlerMock: messageHandlerMock,
		peerPoolMock:       peerPoolMock,
		clientMock:         clientMock,
		ctrl:               ctrl,
	}
}

func (fx *fixture) stop() {
	fx.ctrl.Finish()
}

func TestRequestManager_Request(t *testing.T) {
	ctx := context.Background()

	t.Run("send request", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()

		peerId := "peerId"
		peerMock := mock_peer.NewMockPeer(fx.ctrl)
		conn := &drpcconn.Conn{}
		msg := &spacesyncproto.ObjectSyncMessage{}
		resp := &spacesyncproto.ObjectSyncMessage{}
		fx.peerPoolMock.EXPECT().Get(ctx, peerId).Return(peerMock, nil)
		fx.clientMock.EXPECT().ObjectSync(ctx, msg).Return(resp, nil)
		peerMock.EXPECT().DoDrpc(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, drpcHandler func(conn drpc.Conn) error) {
			drpcHandler(conn)
		}).Return(nil)
		res, err := fx.requestManager.SendRequest(ctx, peerId, msg)
		require.NoError(t, err)
		require.Equal(t, resp, res)
	})

	t.Run("request and handle", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		ctx = fx.requestManager.ctx

		peerId := "peerId"
		peerMock := mock_peer.NewMockPeer(fx.ctrl)
		conn := &drpcconn.Conn{}
		msg := &spacesyncproto.ObjectSyncMessage{}
		resp := &spacesyncproto.ObjectSyncMessage{}
		fx.peerPoolMock.EXPECT().Get(ctx, peerId).Return(peerMock, nil)
		fx.clientMock.EXPECT().ObjectSync(ctx, msg).Return(resp, nil)
		peerMock.EXPECT().DoDrpc(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, drpcHandler func(conn drpc.Conn) error) {
			drpcHandler(conn)
		}).Return(nil)
		fx.messageHandlerMock.EXPECT().HandleMessage(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, msg objectsync.HandleMessage) {
			require.Equal(t, peerId, msg.SenderId)
			require.Equal(t, resp, msg.Message)
			pId, _ := peer.CtxPeerId(msg.PeerCtx)
			require.Equal(t, peerId, pId)
		}).Return(nil)
		fx.requestManager.requestAndHandle(peerId, msg)
	})
}
