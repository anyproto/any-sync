package peer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"storj.io/drpc"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"

	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/encoding"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	pb "github.com/anyproto/any-sync/net/streampool/testservice"
	"github.com/anyproto/any-sync/net/transport"
)

func init() {
	serverConns = make(chan net.Conn, 1000)

	// build the real dRPC server with your generated mux
	mux := drpcmux.New()
	pb.DRPCRegisterTest(mux, &testServer{}) // reuse your testServer from above
	srv := drpcserver.New(encoding.WrapHandler(mux))

	// server loop
	go func() {
		for c := range serverConns {
			go func(c net.Conn) {
				// 1) do the libp2p/TLS handshake
				_, err := handshake.IncomingProtoHandshake(ctx, c, noSnappyProtoChecker)

				if err != nil {
					fmt.Printf("handshake failed: %v\n", err)
				}
				// 2) now serve the dRPC on the secured conn
				_ = srv.ServeOne(context.Background(), c)
			}(c)
		}
	}()
}

func TestPeer_Stream_NotCanceled_NoRead(t *testing.T) {
	// in case of 0 pipe delay, it still reproduces but needs  more iterations
	fx := newPeerFixture(t, "peer‑race", 0)

	var someErr = errors.New("some error")
	for i := 0; i < 100; i++ {
		// start a streaming RPC and cancel mid-send
		//ctxA, _ := context.WithCancel(context.Background())
		err := fx.DoDrpc(context.Background(), func(c drpc.Conn) error {
			client := pb.NewDRPCTestClient(c)
			_, err := client.TestStream2(context.Background(), &pb.StreamMessage{ReqData: "ping", Repeat: 100})
			if err != nil {
				return err
			}
			// do not drain the stream, return some random error
			return someErr
		})

		if !errors.Is(err, someErr) {
			t.Fatalf("iteration %d: expected someErr, got %v", i, err)
		}

		// immediately run a small RPC
		err = fx.DoDrpc(context.Background(), func(c drpc.Conn) error {
			client := pb.NewDRPCTestClient(c)
			strm, err := client.TestStream2(context.Background(), &pb.StreamMessage{ReqData: "ping"})
			if err != nil {
				return err
			}

			return drainStream(context.Background(), strm)
		})
		require.NoError(t, err)
	}
}

func TestPeer_Stream_Canceled_NoRead(t *testing.T) {
	// in case of 0 pipe delay, it still reproduces but needs  more iterations
	fx := newPeerFixture(t, "peer‑race", 300)

	for i := 0; i < 100; i++ {
		// start a streaming RPC and cancel mid-send
		ctxA, cancelA := context.WithCancel(context.Background())
		err := fx.DoDrpc(ctxA, func(c drpc.Conn) error {
			client := pb.NewDRPCTestClient(c)
			_, err := client.TestStream2(ctxA, &pb.StreamMessage{ReqData: "ping", Repeat: 100})
			if err != nil {
				return err
			}
			cancelA()
			// do not drain the stream, pretend that we immediately handle the canceled context
			return ctxA.Err()
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: expected context.Canceled, got %v", i, err)
		}

		// immediately run a small RPC
		err = fx.DoDrpc(context.Background(), func(c drpc.Conn) error {
			client := pb.NewDRPCTestClient(c)
			strm, err := client.TestStream2(context.Background(), &pb.StreamMessage{ReqData: "ping"})
			if err != nil {
				return err
			}

			return drainStream(context.Background(), strm)
		})

		if errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: pool reused killed conn", i)
		}
	}
}

func TestPeer_Stream_Canceled_ReadAll(t *testing.T) {
	fx := newPeerFixture(t, "peer‑race", 50)
	data := strings.Repeat("x", 1024*1000)
	for i := 0; i < 100; i++ {
		// start a streaming RPC and cancel mid-send
		ctxA, cancelA := context.WithCancel(context.Background())

		err := fx.DoDrpc(ctxA, func(c drpc.Conn) error {
			client := pb.NewDRPCTestClient(c)
			strm, err := client.TestStream2(ctxA, &pb.StreamMessage{ReqData: data, Repeat: 100})
			if err != nil {
				return err
			}
			//
			time.Sleep(10 * time.Millisecond)
			cancelA()

			return drainStream(context.Background(), strm)
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: expected context.Canceled, got %v", i, err)
		}

		// immediately run a small RPC
		var conn drpc.Conn
		err = fx.DoDrpc(context.Background(), func(c drpc.Conn) error {
			conn = c
			client := pb.NewDRPCTestClient(c)
			strm, err := client.TestStream2(context.Background(), &pb.StreamMessage{ReqData: "ping", Repeat: 1})
			if err != nil {
				return err
			}

			return drainStream(context.Background(), strm)
		})

		if errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: pool reused killed conn %p", i, conn)
		}
	}
}

/* -------------------------------------------------------------------------- */
/* 1. Server implementation                                                   */
/* -------------------------------------------------------------------------- */

// testServer satisfies pb.TestServer (generated interface).
type testServer struct {
	pb.DRPCTestServer
}

func (ts *testServer) TestStream2(req *pb.StreamMessage,
	ss pb.DRPCTest_TestStream2Stream,
) error {
	// req is the *single* request message sent by the client
	for i := 0; i < int(req.Repeat); i++ {
		err := ss.Send(&pb.StreamMessage{
			ReqData: req.ReqData, // echo back what we got
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var serverConns chan net.Conn

// ─────────────────────────────────────────────────────────────────────────────
// pipeMultiConn: implements transport.MultiConn for in-memory pipe+server
// ─────────────────────────────────────────────────────────────────────────────

type pipeMultiConn struct {
	ctx   context.Context
	delay time.Duration
}

var _ transport.MultiConn = (*pipeMultiConn)(nil)

func (p *pipeMultiConn) Context() context.Context   { return p.ctx }
func (p *pipeMultiConn) Addr() string               { return "pipe" }
func (p *pipeMultiConn) Close() error               { return nil }
func (p *pipeMultiConn) CloseChan() <-chan struct{} { ch := make(chan struct{}); close(ch); return ch }
func (p *pipeMultiConn) IsClosed() bool             { return false }
func (p *pipeMultiConn) Accept() (net.Conn, error)  { return nil, transport.ErrConnClosed }

func (p *pipeMultiConn) Open(ctx context.Context) (net.Conn, error) {
	client, server := NewDelayPipe(p.delay)
	serverConns <- server
	return client, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// dummyCtrl satisfies connCtrl for outgoing-only pool
// ─────────────────────────────────────────────────────────────────────────────

type dummyCtrl struct{}

func (dummyCtrl) DrpcConfig() rpc.Config {
	return rpc.Config{Stream: rpc.StreamConfig{MaxMsgSizeMb: 1}}
}
func (dummyCtrl) ServeConn(ctx context.Context, conn net.Conn) error { return io.EOF }

// ─────────────────────────────────────────────────────────────────────────────
// peerFixture bundles pool + transport for reuse in multiple tests
// ─────────────────────────────────────────────────────────────────────────────

type peerFixture struct {
	t    *testing.T
	peer *peer
}

func newPeerFixture(t *testing.T, peerId string, pipeDelay time.Duration) *peerFixture {
	p, err := NewPeer(&pipeMultiConn{ctx: CtxWithPeerId(context.Background(), peerId), delay: pipeDelay}, dummyCtrl{})
	require.NoError(t, err)
	return &peerFixture{t: t, peer: p.(*peer)}
}

func (f *peerFixture) DoDrpc(ctx context.Context, fn func(c drpc.Conn) error) error {
	return f.peer.DoDrpc(ctx, fn)
}

func drainStream(ctx context.Context, stream pb.DRPCTest_TestStream2Client) error {
	var err error
	// DRAIN until EOF
	for {
		_, err = stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err // either io.EOF or a real error
		}
		// mimic our collector logic
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:

		}
	}
}

// delayConn wraps a net.Conn and delays each Read and Write by d.
type delayConn struct {
	net.Conn
	d time.Duration
}

func (c *delayConn) Read(b []byte) (n int, err error) {
	if c.d > 0 {
		time.Sleep(c.d)
	}
	return c.Conn.Read(b)
}

func (c *delayConn) Write(b []byte) (n int, err error) {
	if c.d > 0 {
		time.Sleep(c.d)
	}
	return c.Conn.Write(b)
}

// NewDelayPipe returns two endpoints of a in-memory pipe that each
// delay all Reads and Writes by the given duration.
func NewDelayPipe(d time.Duration) (net.Conn, net.Conn) {
	a, b := net.Pipe()
	return &delayConn{Conn: a, d: d}, &delayConn{Conn: b, d: d}
}
