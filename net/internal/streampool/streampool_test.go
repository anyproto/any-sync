package streampool

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/net/internal/peer"
	"github.com/anyproto/any-sync/net/internal/rpc/rpctest"
	"github.com/anyproto/any-sync/net/internal/streampool/testservice"
	peer2 "github.com/anyproto/any-sync/net/peer"
	streampool2 "github.com/anyproto/any-sync/net/streampool"
)

var ctx = context.Background()

func makePeerPair(t *testing.T, fx *fixture, peerId string) (pS, pC peer2.Peer) {
	mcS, mcC := rpctest.MultiConnPair(peerId+"server", peerId)
	pS, err := peer.NewPeer(mcS, fx.ts)
	require.NoError(t, err)
	pC, err = peer.NewPeer(mcC, fx.ts)
	require.NoError(t, err)
	return
}

func newClientStream(t *testing.T, fx *fixture, peerId string) (st testservice.DRPCTest_TestStreamClient, p peer.Peer) {
	_, pC := makePeerPair(t, fx, peerId)
	drpcConn, err := pC.AcquireDrpcConn(ctx)
	require.NoError(t, err)
	st, err = testservice.NewDRPCTestClient(drpcConn).TestStream(pC.Context())
	require.NoError(t, err)
	return st, pC
}

func TestStreamPool_AddStream(t *testing.T) {
	t.Run("broadcast incoming", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)

		s1, _ := newClientStream(t, fx, "p1")
		require.NoError(t, fx.AddStream(s1, "space1", "common"))
		s2, _ := newClientStream(t, fx, "p2")
		require.NoError(t, fx.AddStream(s2, "space2", "common"))

		require.NoError(t, fx.Broadcast(ctx, &testservice.StreamMessage{ReqData: "space1"}, "space1"))
		require.NoError(t, fx.Broadcast(ctx, &testservice.StreamMessage{ReqData: "space2"}, "space2"))
		require.NoError(t, fx.Broadcast(ctx, &testservice.StreamMessage{ReqData: "common"}, "common"))

		var serverResults []string
		for i := 0; i < 4; i++ {
			select {
			case msg := <-fx.tsh.receiveCh:
				serverResults = append(serverResults, msg.ReqData)
			case <-time.After(time.Second):
				require.NoError(t, fmt.Errorf("timeout"))
			}
		}

		sort.Strings(serverResults)
		assert.Equal(t, []string{"common", "common", "space1", "space2"}, serverResults)

		assert.NoError(t, s1.Close())
		assert.NoError(t, s2.Close())
	})

	t.Run("send incoming", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)

		s1, p1 := newClientStream(t, fx, "p1")
		defer s1.Close()
		require.NoError(t, fx.AddStream(s1, "space1", "common"))

		require.NoError(t, fx.Send(ctx, &testservice.StreamMessage{ReqData: "test"}, func(ctx context.Context) (peers []peer.Peer, err error) {
			return []peer.Peer{p1}, nil
		}))
		var msg *testservice.StreamMessage
		select {
		case msg = <-fx.tsh.receiveCh:
		case <-time.After(time.Second):
			require.NoError(t, fmt.Errorf("timeout"))
		}
		assert.Equal(t, "test", msg.ReqData)
	})
}

func TestStreamPool_Send(t *testing.T) {
	t.Run("open stream", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)

		pS, _ := makePeerPair(t, fx, "p1")

		require.NoError(t, fx.Send(ctx, &testservice.StreamMessage{ReqData: "should open stream"}, func(ctx context.Context) (peers []peer.Peer, err error) {
			return []peer.Peer{pS}, nil
		}))

		var msg *testservice.StreamMessage
		select {
		case msg = <-fx.tsh.receiveCh:
		case <-time.After(time.Second):
			require.NoError(t, fmt.Errorf("timeout"))
		}
		assert.Equal(t, "should open stream", msg.ReqData)
	})

	t.Run("parallel open stream", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)

		pS, _ := makePeerPair(t, fx, "p1")

		fx.th.streamOpenDelay = time.Second / 3

		var numMsgs = 5

		for i := 0; i < numMsgs; i++ {
			go require.NoError(t, fx.Send(ctx, &testservice.StreamMessage{ReqData: "should open stream"}, func(ctx context.Context) (peers []peer.Peer, err error) {
				return []peer.Peer{pS}, nil
			}))
		}

		var msgs []*testservice.StreamMessage
		for i := 0; i < numMsgs; i++ {
			select {
			case msg := <-fx.tsh.receiveCh:
				msgs = append(msgs, msg)
			case <-time.After(time.Second):
				require.NoError(t, fmt.Errorf("timeout"))
			}
		}
		assert.Len(t, msgs, numMsgs)
		// make sure that we have only one stream
		assert.Equal(t, int32(1), fx.tsh.streamsCount.Load())
	})
	t.Run("parallel open stream error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)

		pS, _ := makePeerPair(t, fx, "p1")
		_ = pS.Close()

		fx.th.streamOpenDelay = time.Second / 3

		var numMsgs = 5

		var wg sync.WaitGroup
		for i := 0; i < numMsgs; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.Error(t, fx.StreamPool.(*streamPool).sendOne(ctx, pS, &testservice.StreamMessage{ReqData: "should open stream"}))
			}()
		}
		wg.Wait()
	})

}

func TestStreamPool_SendById(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)

	s1, _ := newClientStream(t, fx, "p1")
	defer s1.Close()
	require.NoError(t, fx.AddStream(s1, "space1", "common"))

	require.NoError(t, fx.SendById(ctx, &testservice.StreamMessage{ReqData: "test"}, "p1"))
	var msg *testservice.StreamMessage
	select {
	case msg = <-fx.tsh.receiveCh:
	case <-time.After(time.Second):
		require.NoError(t, fmt.Errorf("timeout"))
	}
	assert.Equal(t, "test", msg.ReqData)
}

func TestStreamPool_Tags(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)

	s1, _ := newClientStream(t, fx, "p1")
	defer s1.Close()
	require.NoError(t, fx.AddStream(s1, "t1"))

	s2, _ := newClientStream(t, fx, "p2")
	defer s1.Close()
	require.NoError(t, fx.AddStream(s2, "t2"))

	err := fx.AddTagsCtx(streamCtx(ctx, 1, "p1"), "t3", "t3")
	require.NoError(t, err)
	assert.Equal(t, []uint32{1}, fx.StreamPool.(*streamPool).streamIdsByTag["t3"])

	err = fx.RemoveTagsCtx(streamCtx(ctx, 2, "p2"), "t2")
	require.NoError(t, err)
	assert.Len(t, fx.StreamPool.(*streamPool).streamIdsByTag["t2"], 0)

}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{}
	fx.ts = rpctest.NewTestServer()
	fx.tsh = &testServerHandler{receiveCh: make(chan *testservice.StreamMessage, 100)}
	require.NoError(t, testservice.DRPCRegisterTest(fx.ts, fx.tsh))
	fx.th = &testHandler{}
	s := New()
	s.(*service).debugStat = debugstat.NewNoOp()
	fx.StreamPool = s.NewStreamPool(fx.th, streampool2.StreamConfig{
		SendQueueSize:    10,
		DialQueueWorkers: 1,
		DialQueueSize:    10,
	})
	return fx
}

type fixture struct {
	StreamPool
	th  *testHandler
	tsh *testServerHandler
	ts  *rpctest.TestServer
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.Close())
}

type testHandler struct {
	streamOpenDelay  time.Duration
	incomingMessages []drpc.Message
	mu               sync.Mutex
}

func (t *testHandler) OpenStream(ctx context.Context, p peer.Peer) (stream drpc.Stream, tags []string, err error) {
	if t.streamOpenDelay > 0 {
		time.Sleep(t.streamOpenDelay)
	}
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		return
	}
	stream, err = testservice.NewDRPCTestClient(conn).TestStream(p.Context())
	return
}

func (t *testHandler) HandleMessage(ctx context.Context, peerId string, msg drpc.Message) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.incomingMessages = append(t.incomingMessages, msg)
	return nil
}

func (t *testHandler) DRPCEncoding() drpc.Encoding {
	return EncodingProto
}

func (t *testHandler) NewReadMessage() drpc.Message {
	return new(testservice.StreamMessage)
}

type testServerHandler struct {
	receiveCh    chan *testservice.StreamMessage
	streamsCount atomic.Int32
	mu           sync.Mutex
}

func (t *testServerHandler) TestStream(st testservice.DRPCTest_TestStreamStream) error {
	t.streamsCount.Add(1)
	defer t.streamsCount.Add(-1)
	for {
		msg, err := st.Recv()
		if err != nil {
			return err
		}
		t.receiveCh <- msg
		if err = st.Send(msg); err != nil {
			return err
		}
	}
	return nil
}
