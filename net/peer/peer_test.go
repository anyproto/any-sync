package peer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	_ "net/http/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcwire"

	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/transport/mock_transport"
)

var ctx = context.Background()

type streamRawWrite interface {
	RawWrite(kind drpcwire.Kind, data []byte) (err error)
}

func TestPeer_ReusesConnAfterHardCancel(t *testing.T) {
	fx := newFixture(t, "peer‑race")
	defer fx.finish()

	fx.mc.EXPECT().
		Open(gomock.Any()).
		DoAndReturn(func(_ context.Context) (net.Conn, error) {
			in, out := net.Pipe()
			go handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker)
			return in, nil
		}).
		AnyTimes()

	for i := 0; i < 10; i++ {
		fmt.Printf("%s ### Iteration %d\n", time.Now().Format(time.StampMilli), i)
		// 2. Immediately try another RPC on the same pool.
		ctxB, cancelB := context.WithCancel(context.Background())
		err := fx.DoDrpc(ctxB, func(c drpc.Conn) error {
			stream, err := c.NewStream(ctxB, "/race.Test/Ping", nil)
			if err != nil {
				return fmt.Errorf("failed to create stream: %w", err)
			}
			streamRaw := stream.(streamRawWrite)
			err = streamRaw.RawWrite(drpcwire.KindMessage, []byte("ping"))
			if err != nil {
				return fmt.Errorf("failed to send message: %w", err)
			}
			return nil
		})
		cancelB()

		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("iteration %d: hung waiting for semaphore (bug aggravated)", i)
		}
		if errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: got the spurious “context canceled”", i)
		}
	}
}

func TestPeer_AcquireDrpcConn(t *testing.T) {
	t.Run("generic", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		in, out := net.Pipe()
		go func() {
			handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker)
		}()
		defer out.Close()
		fx.mc.EXPECT().Open(gomock.Any()).Return(in, nil)
		dc, err := fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, dc)
		defer dc.Close()

		assert.Len(t, fx.active, 1)
		assert.Len(t, fx.inactive, 0)

		fx.ReleaseDrpcConn(ctx, dc)

		assert.Len(t, fx.active, 0)
		assert.Len(t, fx.inactive, 1)

		dc, err = fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, dc)
		assert.Len(t, fx.active, 1)
		assert.Len(t, fx.inactive, 0)
	})
	t.Run("closed sub conn", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()

		closedIn, _ := net.Pipe()
		dc := drpcconn.New(closedIn)
		fx.ReleaseDrpcConn(ctx, &subConn{ConnUnblocked: dc})
		dc.Close()

		in, out := net.Pipe()
		go func() {
			handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker)
		}()
		defer out.Close()
		fx.mc.EXPECT().Open(gomock.Any()).Return(in, nil)
		_, err := fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)
	})
}

func TestPeer_DrpcConn_OpenThrottling(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()

	acquire := func() (func(), drpc.Conn, error) {
		in, out := net.Pipe()
		go func() {
			_, err := handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker)
			require.NoError(t, err)
		}()

		fx.mc.EXPECT().Open(gomock.Any()).Return(in, nil)
		dconn, err := fx.AcquireDrpcConn(ctx)
		return func() { out.Close() }, dconn, err
	}

	var conCount = fx.limiter.startThreshold + 3
	var conns []drpc.Conn
	for i := 0; i < conCount; i++ {
		cc, dc, err := acquire()
		require.NoError(t, err)
		defer cc()
		conns = append(conns, dc)
	}

	go func() {
		time.Sleep(fx.limiter.slowDownStep)
		fx.ReleaseDrpcConn(ctx, conns[0])
		conns = conns[1:]
	}()
	_, err := fx.AcquireDrpcConn(ctx)
	require.NoError(t, err)
}

func TestPeerAccept(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	in, out := net.Pipe()
	defer out.Close()

	var outHandshakeCh = make(chan error)
	go func() {
		_, hErr := handshake.OutgoingProtoHandshake(ctx, out, defaultHandshakeProto)
		outHandshakeCh <- hErr
	}()
	fx.acceptCh <- acceptedConn{conn: in}
	cn := <-fx.testCtrl.serveConn
	assert.Equal(t, in, cn)
	assert.NoError(t, <-outHandshakeCh)
}

func TestPeer_DrpcConn_AcceptThrottling(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()

	var conCount = fx.limiter.startThreshold + 3
	for i := 0; i < conCount; i++ {
		in, out := net.Pipe()
		defer out.Close()

		var outHandshakeCh = make(chan error)
		go func() {
			_, hErr := handshake.OutgoingProtoHandshake(ctx, out, defaultHandshakeProto)
			outHandshakeCh <- hErr
		}()
		fx.acceptCh <- acceptedConn{conn: in}
		cn := <-fx.testCtrl.serveConn
		assert.Equal(t, in, cn)
		assert.NoError(t, <-outHandshakeCh)
	}
}

func TestPeer_TryClose(t *testing.T) {
	t.Run("not close in first minute", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		res, err := fx.TryClose(time.Second)
		require.NoError(t, err)
		assert.False(t, res)
	})
	t.Run("close", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		fx.peer.created = fx.peer.created.Add(-time.Minute * 2)
		res, err := fx.TryClose(time.Second)
		require.NoError(t, err)
		assert.True(t, res)
	})
	t.Run("custom ttl", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		fx.peer.created = fx.peer.created.Add(-time.Minute * 2)
		fx.peer.SetTTL(time.Hour)

		in, out := net.Pipe()
		hsDone := make(chan struct{})
		go func() {
			defer close(hsDone)
			handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker)
		}()
		defer out.Close()
		fx.mc.EXPECT().Open(gomock.Any()).Return(in, nil)
		fx.mc.EXPECT().Addr().AnyTimes()
		_, err := fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)
		time.Sleep(time.Second)
		res, err := fx.TryClose(time.Second / 2)
		require.NoError(t, err)
		assert.False(t, res)
	})
	t.Run("gc", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		//now := time.Now()

		// make one inactive
		in, out := net.Pipe()
		go func() {
			handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker)
		}()
		defer out.Close()
		fx.mc.EXPECT().Open(gomock.Any()).Return(in, nil)
		dc, err := fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)

		// make one active but closed
		in2, out2 := net.Pipe()
		go func() {
			handshake.IncomingProtoHandshake(ctx, out2, defaultProtoChecker)
		}()
		defer out2.Close()
		fx.mc.EXPECT().Open(gomock.Any()).Return(in2, nil)
		fx.mc.EXPECT().Addr().Return("")
		dc2, err := fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)
		_ = dc2.Close()

		// make one inactive and closed
		in3, out3 := net.Pipe()
		go func() {
			handshake.IncomingProtoHandshake(ctx, out3, defaultProtoChecker)
		}()
		defer out3.Close()
		fx.mc.EXPECT().Open(gomock.Any()).Return(in3, nil)
		dc3, err := fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)

		// make another active, should be removed by ttl
		in4, out4 := net.Pipe()
		go func() {
			handshake.IncomingProtoHandshake(ctx, out4, defaultProtoChecker)
		}()
		defer out4.Close()
		fx.mc.EXPECT().Open(gomock.Any()).Return(in4, nil)
		dc4, err := fx.AcquireDrpcConn(ctx)
		require.NoError(t, err)
		defer dc4.Close()

		fx.ReleaseDrpcConn(ctx, dc3)
		_ = dc3.Close()
		fx.ReleaseDrpcConn(ctx, dc)

		time.Sleep(time.Millisecond * 100)

		res, err := fx.TryClose(time.Millisecond * 50)
		require.NoError(t, err)
		assert.False(t, res)
	})
}

type acceptedConn struct {
	conn net.Conn
	err  error
}

func newFixture(t *testing.T, peerId string) *fixture {
	fx := &fixture{
		ctrl:     gomock.NewController(t),
		acceptCh: make(chan acceptedConn),
		testCtrl: newTesCtrl(),
	}
	fx.mc = mock_transport.NewMockMultiConn(fx.ctrl)
	ctx := CtxWithPeerId(context.Background(), peerId)
	fx.mc.EXPECT().Context().Return(ctx).AnyTimes()
	fx.mc.EXPECT().Accept().DoAndReturn(func() (net.Conn, error) {
		ac := <-fx.acceptCh
		return ac.conn, ac.err
	}).AnyTimes()
	fx.mc.EXPECT().Close().AnyTimes()
	p, err := NewPeer(fx.mc, fx.testCtrl)
	require.NoError(t, err)
	fx.peer = p.(*peer)
	return fx
}

type fixture struct {
	*peer
	ctrl     *gomock.Controller
	mc       *mock_transport.MockMultiConn
	acceptCh chan acceptedConn
	testCtrl *testCtrl
}

func (fx *fixture) finish() {
	fx.testCtrl.close()
	fx.ctrl.Finish()
}

func newTesCtrl() *testCtrl {
	return &testCtrl{closeCh: make(chan struct{}), serveConn: make(chan net.Conn, 10)}
}

type testCtrl struct {
	serveConn chan net.Conn
	closeCh   chan struct{}
}

func (t *testCtrl) DrpcConfig() rpc.Config {
	return rpc.Config{Stream: rpc.StreamConfig{MaxMsgSizeMb: 10}}
}

func (t *testCtrl) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	t.serveConn <- conn
	<-t.closeCh
	return io.EOF
}

func (t *testCtrl) close() {
	close(t.closeCh)
}
