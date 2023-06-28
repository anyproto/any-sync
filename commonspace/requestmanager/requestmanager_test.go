package requestmanager

import (
	"context"
	"sync"
	"testing"

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

func TestRequestManager_SyncRequest(t *testing.T) {
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

func TestRequestManager_QueueRequest(t *testing.T) {
	t.Run("max concurrent reqs for peer, independent reqs for other peer", func(t *testing.T) {
		// testing 2 concurrent requests to one peer and simultaneous to another peer
		fx := newFixture(t)
		defer fx.stop()
		fx.requestManager.workers = 2
		msgRelease := make(chan struct{})
		msgWait := make(chan struct{})
		msgs := sync.Map{}
		doRequestAndHandle = func(manager *requestManager, peerId string, req *spacesyncproto.ObjectSyncMessage) {
			msgs.Store(req.ObjectId, struct{}{})
			<-msgWait
			<-msgRelease
		}
		otherPeer := "otherPeer"
		msg1 := &spacesyncproto.ObjectSyncMessage{ObjectId: "id1"}
		msg2 := &spacesyncproto.ObjectSyncMessage{ObjectId: "id2"}
		msg3 := &spacesyncproto.ObjectSyncMessage{ObjectId: "id3"}
		otherMsg1 := &spacesyncproto.ObjectSyncMessage{ObjectId: "otherId1"}

		// sending requests to first peer
		peerId := "peerId"
		err := fx.requestManager.QueueRequest(peerId, msg1)
		require.NoError(t, err)
		err = fx.requestManager.QueueRequest(peerId, msg2)
		require.NoError(t, err)
		err = fx.requestManager.QueueRequest(peerId, msg3)
		require.NoError(t, err)

		// waiting until all the messages are loaded
		msgWait <- struct{}{}
		msgWait <- struct{}{}
		_, ok := msgs.Load("id1")
		require.True(t, ok)
		_, ok = msgs.Load("id2")
		require.True(t, ok)
		// third message should not be read
		_, ok = msgs.Load("id3")
		require.False(t, ok)

		// request for other peer should pass
		err = fx.requestManager.QueueRequest(otherPeer, otherMsg1)
		require.NoError(t, err)
		msgWait <- struct{}{}

		_, ok = msgs.Load("otherId1")
		require.True(t, ok)
		close(msgRelease)
		fx.requestManager.Close(context.Background())
	})

	t.Run("no requests after close", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.stop()
		fx.requestManager.workers = 1
		msgRelease := make(chan struct{})
		msgWait := make(chan struct{})
		msgs := sync.Map{}
		doRequestAndHandle = func(manager *requestManager, peerId string, req *spacesyncproto.ObjectSyncMessage) {
			msgs.Store(req.ObjectId, struct{}{})
			<-msgWait
			<-msgRelease
		}
		msg1 := &spacesyncproto.ObjectSyncMessage{ObjectId: "id1"}
		msg2 := &spacesyncproto.ObjectSyncMessage{ObjectId: "id2"}

		// sending requests to first peer
		peerId := "peerId"
		err := fx.requestManager.QueueRequest(peerId, msg1)
		require.NoError(t, err)
		err = fx.requestManager.QueueRequest(peerId, msg2)
		require.NoError(t, err)

		// waiting until all the message is loaded
		msgWait <- struct{}{}
		_, ok := msgs.Load("id1")
		require.True(t, ok)
		_, ok = msgs.Load("id2")
		require.False(t, ok)

		fx.requestManager.Close(context.Background())
		close(msgRelease)
		_, ok = msgs.Load("id2")
		require.False(t, ok)
	})
}
