package objectsync

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/rpc/rpctest"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testServer struct {
	stream        chan spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream
	addLog        func(ctx context.Context, req *consensusproto.AddLogRequest) error
	addRecord     func(ctx context.Context, req *consensusproto.AddRecordRequest) error
	releaseStream chan error
	watchErrOnce  bool
}

func (t *testServer) HeadSync(ctx context.Context, request *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	panic("implement me")
}

func (t *testServer) SpacePush(ctx context.Context, request *spacesyncproto.SpacePushRequest) (*spacesyncproto.SpacePushResponse, error) {
	panic("implement me")
}

func (t *testServer) SpacePull(ctx context.Context, request *spacesyncproto.SpacePullRequest) (*spacesyncproto.SpacePullResponse, error) {
	panic("implement me")
}

func (t *testServer) ObjectSyncStream(stream spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream) error {
	t.stream <- stream
	return <-t.releaseStream
}

func (t *testServer) waitStream(test *testing.T) spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream {
	select {
	case <-time.After(time.Second * 5):
		test.Fatalf("waiteStream timeout")
	case st := <-t.stream:
		return st
	}
	return nil
}

type fixture struct {
	testServer   *testServer
	drpcTS       *rpctest.TesServer
	client       spacesyncproto.DRPCSpaceSyncClient
	clientStream spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream
	serverStream spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream
	pool         *streamPool
	clientId     string
	serverId     string
}

func newFixture(t *testing.T, clientId, serverId string, handler MessageHandler) *fixture {
	fx := &fixture{
		testServer: &testServer{},
		drpcTS:     rpctest.NewTestServer(),
		clientId:   clientId,
		serverId:   serverId,
	}
	fx.testServer.stream = make(chan spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream, 1)
	require.NoError(t, spacesyncproto.DRPCRegisterSpaceSync(fx.drpcTS.Mux, fx.testServer))
	fx.client = spacesyncproto.NewDRPCSpaceSyncClient(fx.drpcTS.Dial(peer.CtxWithPeerId(context.Background(), clientId)))

	var err error
	fx.clientStream, err = fx.client.ObjectSyncStream(peer.CtxWithPeerId(context.Background(), serverId))
	require.NoError(t, err)
	fx.serverStream = fx.testServer.waitStream(t)
	fx.pool = newStreamPool(handler).(*streamPool)

	return fx
}

func (fx *fixture) run(t *testing.T) chan error {
	waitCh := make(chan error)
	go func() {
		err := fx.pool.AddAndReadStreamSync(fx.clientStream)
		waitCh <- err
	}()

	time.Sleep(time.Millisecond * 10)
	fx.pool.Lock()
	require.Equal(t, fx.pool.peerStreams[fx.serverId], fx.clientStream)
	fx.pool.Unlock()

	return waitCh
}

func TestStreamPool_AddAndReadStreamAsync(t *testing.T) {
	remId := "remoteId"

	t.Run("client close", func(t *testing.T) {
		fx := newFixture(t, "", remId, nil)
		waitCh := fx.run(t)

		err := fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh

		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})
	t.Run("server close", func(t *testing.T) {
		fx := newFixture(t, "", remId, nil)
		waitCh := fx.run(t)

		err := fx.serverStream.Close()
		require.NoError(t, err)

		err = <-waitCh
		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})
}

func TestStreamPool_Close(t *testing.T) {
	remId := "remoteId"

	t.Run("close", func(t *testing.T) {
		fx := newFixture(t, "", remId, nil)
		fx.run(t)
		fx.pool.Close()
		select {
		case <-fx.clientStream.Context().Done():
			break
		case <-time.After(time.Millisecond * 100):
			t.Fatal("context should be closed")
		}
	})
}

func TestStreamPool_ReceiveMessage(t *testing.T) {
	remId := "remoteId"
	t.Run("pool receive message from server", func(t *testing.T) {
		objectId := "objectId"
		msg := &spacesyncproto.ObjectSyncMessage{
			ObjectId: objectId,
		}
		recvChan := make(chan struct{})
		fx := newFixture(t, "", remId, func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
			require.Equal(t, msg, message)
			recvChan <- struct{}{}
			return nil
		})
		waitCh := fx.run(t)

		err := fx.serverStream.Send(msg)
		require.NoError(t, err)
		<-recvChan
		err = fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh

		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})
}

func TestStreamPool_HasActiveStream(t *testing.T) {
	remId := "remoteId"
	t.Run("pool has active stream", func(t *testing.T) {
		fx := newFixture(t, "", remId, nil)
		waitCh := fx.run(t)
		require.True(t, fx.pool.HasActiveStream(remId))

		err := fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh

		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})
	t.Run("pool has no active stream", func(t *testing.T) {
		fx := newFixture(t, "", remId, nil)
		waitCh := fx.run(t)
		err := fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh
		require.Error(t, err)
		require.False(t, fx.pool.HasActiveStream(remId))
		require.Nil(t, fx.pool.peerStreams[remId])
	})
}

func TestStreamPool_SendAsync(t *testing.T) {
	remId := "remoteId"
	t.Run("pool send async to server", func(t *testing.T) {
		objectId := "objectId"
		msg := &spacesyncproto.ObjectSyncMessage{
			ObjectId: objectId,
		}
		fx := newFixture(t, "", remId, nil)
		recvChan := make(chan struct{})
		go func() {
			message, err := fx.serverStream.Recv()
			require.NoError(t, err)
			require.Equal(t, msg, message)
			recvChan <- struct{}{}
		}()
		waitCh := fx.run(t)

		err := fx.pool.SendAsync([]string{remId}, msg)
		require.NoError(t, err)
		<-recvChan
		err = fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh

		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})
}

func TestStreamPool_SendSync(t *testing.T) {
	remId := "remoteId"
	t.Run("pool send sync to server", func(t *testing.T) {
		objectId := "objectId"
		payload := []byte("payload")
		msg := &spacesyncproto.ObjectSyncMessage{
			ObjectId: objectId,
		}
		fx := newFixture(t, "", remId, nil)
		go func() {
			message, err := fx.serverStream.Recv()
			require.NoError(t, err)
			require.Equal(t, msg.ObjectId, message.ObjectId)
			require.NotEmpty(t, message.ReplyId)
			message.Payload = payload
			err = fx.serverStream.Send(message)
			require.NoError(t, err)
		}()
		waitCh := fx.run(t)
		res, err := fx.pool.SendSync(remId, msg)
		require.NoError(t, err)
		require.Equal(t, payload, res.Payload)
		err = fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh

		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})

	t.Run("pool send sync timeout", func(t *testing.T) {
		objectId := "objectId"
		msg := &spacesyncproto.ObjectSyncMessage{
			ObjectId: objectId,
		}
		fx := newFixture(t, "", remId, nil)
		syncWaitPeriod = time.Millisecond * 30
		go func() {
			message, err := fx.serverStream.Recv()
			require.NoError(t, err)
			require.Equal(t, msg.ObjectId, message.ObjectId)
			require.NotEmpty(t, message.ReplyId)
		}()
		waitCh := fx.run(t)
		_, err := fx.pool.SendSync(remId, msg)
		require.Equal(t, ErrSyncTimeout, err)
		err = fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh

		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})
}

func TestStreamPool_BroadcastAsync(t *testing.T) {
	remId := "remoteId"
	t.Run("pool broadcast async to server", func(t *testing.T) {
		objectId := "objectId"
		msg := &spacesyncproto.ObjectSyncMessage{
			ObjectId: objectId,
		}
		fx := newFixture(t, "", remId, nil)
		recvChan := make(chan struct{})
		go func() {
			message, err := fx.serverStream.Recv()
			require.NoError(t, err)
			require.Equal(t, msg, message)
			recvChan <- struct{}{}
		}()
		waitCh := fx.run(t)

		err := fx.pool.BroadcastAsync(msg)
		require.NoError(t, err)
		<-recvChan
		err = fx.clientStream.Close()
		require.NoError(t, err)
		err = <-waitCh

		require.Error(t, err)
		require.Nil(t, fx.pool.peerStreams[remId])
	})
}
