package peer_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/rpctest/multiconntest"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/net/transport"
)

// silentCtrl serves sub conns by reading requests and never answering
type silentCtrl struct{}

func (silentCtrl) DrpcConfig() rpc.Config {
	return rpc.Config{Stream: rpc.StreamConfig{MaxMsgSizeMb: 1}}
}

func (silentCtrl) ServeConn(ctx context.Context, conn net.Conn) error {
	_, err := io.Copy(io.Discard, conn)
	return err
}

// closingCtrl ends each sub stream after its first request: a remote that
// closes the sub stream on a live session
type closingCtrl struct{}

func (closingCtrl) DrpcConfig() rpc.Config {
	return rpc.Config{Stream: rpc.StreamConfig{MaxMsgSizeMb: 1}}
}

func (closingCtrl) ServeConn(ctx context.Context, conn net.Conn) error {
	_, _ = conn.Read(make([]byte, 1))
	time.Sleep(50 * time.Millisecond)
	return conn.Close()
}

// TestPeer_RPCOnDeadYamuxSessionIsConnClosed: over real yamux, an RPC in
// flight when the session dies fails with transport.ErrConnClosed, not the
// context.Canceled drpc makes of the stream's io.EOF; a caller's own
// cancellation stays context.Canceled
func TestPeer_RPCOnDeadYamuxSessionIsConnClosed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		close func(serv, client transport.MultiConn)
	}{
		{"remote close", func(serv, _ transport.MultiConn) { _ = serv.Close() }},
		{"local close", func(_, client transport.MultiConn) { _ = client.Close() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mcS, mcC := multiconntest.MultiConnPair(
				peer.CtxWithPeerId(context.Background(), "client"),
				peer.CtxWithPeerId(context.Background(), "server"),
			)
			_, err := peer.NewPeer(mcS, silentCtrl{})
			require.NoError(t, err)
			pr, err := peer.NewPeer(mcC, silentCtrl{})
			require.NoError(t, err)
			defer pr.Close()

			dc, err := pr.AcquireDrpcConn(context.Background())
			require.NoError(t, err)
			res := make(chan error, 1)
			go func() {
				res <- dc.Invoke(context.Background(), "/x/y", nil, &handshakeproto.Proto{Proto: 1}, &handshakeproto.Proto{})
			}()
			time.Sleep(100 * time.Millisecond)
			tc.close(mcS, mcC)
			select {
			case err = <-res:
			case <-time.After(10 * time.Second):
				t.Fatal("the RPC did not return")
			}
			assert.ErrorIs(t, err, transport.ErrConnClosed)
			assert.False(t, errors.Is(err, context.Canceled), "must not look like the caller's cancellation: %v", err)
		})
	}
	t.Run("sub conn closed locally mid-RPC", func(t *testing.T) {
		// what gc or a release does to a sub conn while an RPC runs on it
		mcS, mcC := multiconntest.MultiConnPair(
			peer.CtxWithPeerId(context.Background(), "client"),
			peer.CtxWithPeerId(context.Background(), "server"),
		)
		_, err := peer.NewPeer(mcS, silentCtrl{})
		require.NoError(t, err)
		pr, err := peer.NewPeer(mcC, silentCtrl{})
		require.NoError(t, err)
		defer pr.Close()
		dc, err := pr.AcquireDrpcConn(context.Background())
		require.NoError(t, err)
		res := make(chan error, 1)
		go func() {
			res <- dc.Invoke(context.Background(), "/x/y", nil, &handshakeproto.Proto{Proto: 1}, &handshakeproto.Proto{})
		}()
		time.Sleep(100 * time.Millisecond)
		go func() { _ = dc.Close() }()
		select {
		case err = <-res:
		case <-time.After(10 * time.Second):
			t.Fatal("the RPC did not return")
		}
		assert.ErrorIs(t, err, transport.ErrConnClosed)
		assert.False(t, mcC.IsClosed(), "only the sub conn closed")
	})
	t.Run("remote ends only the sub stream", func(t *testing.T) {
		mcS, mcC := multiconntest.MultiConnPair(
			peer.CtxWithPeerId(context.Background(), "client"),
			peer.CtxWithPeerId(context.Background(), "server"),
		)
		_, err := peer.NewPeer(mcS, closingCtrl{})
		require.NoError(t, err)
		pr, err := peer.NewPeer(mcC, silentCtrl{})
		require.NoError(t, err)
		defer pr.Close()
		dc, err := pr.AcquireDrpcConn(context.Background())
		require.NoError(t, err)
		res := make(chan error, 1)
		go func() {
			res <- dc.Invoke(context.Background(), "/x/y", nil, &handshakeproto.Proto{Proto: 1}, &handshakeproto.Proto{})
		}()
		select {
		case err = <-res:
		case <-time.After(10 * time.Second):
			t.Fatal("the RPC did not return")
		}
		assert.ErrorIs(t, err, transport.ErrConnClosed)
		assert.False(t, errors.Is(err, context.Canceled), "must not look like the caller's cancellation: %v", err)
		assert.False(t, mcC.IsClosed(), "the session is alive")
	})
	t.Run("caller cancel stays canceled", func(t *testing.T) {
		mcS, mcC := multiconntest.MultiConnPair(
			peer.CtxWithPeerId(context.Background(), "client"),
			peer.CtxWithPeerId(context.Background(), "server"),
		)
		_, err := peer.NewPeer(mcS, silentCtrl{})
		require.NoError(t, err)
		pr, err := peer.NewPeer(mcC, silentCtrl{})
		require.NoError(t, err)
		defer pr.Close()

		dc, err := pr.AcquireDrpcConn(context.Background())
		require.NoError(t, err)
		cctx, cancel := context.WithCancel(context.Background())
		res := make(chan error, 1)
		go func() {
			res <- dc.Invoke(cctx, "/x/y", nil, &handshakeproto.Proto{Proto: 1}, &handshakeproto.Proto{})
		}()
		time.Sleep(100 * time.Millisecond)
		cancel()
		select {
		case err = <-res:
		case <-time.After(10 * time.Second):
			t.Fatal("the RPC did not return")
		}
		assert.ErrorIs(t, err, context.Canceled)
		assert.NotErrorIs(t, err, transport.ErrConnClosed)
	})
}
