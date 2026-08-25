package iroh

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	goiroh "github.com/tmc/go-iroh/iroh"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

const (
	closeCodeNormal    = 0
	closeCodeRefused   = 2
	closeCodeHandshake = 3
)

func newConn(cctx context.Context, conn *goiroh.Conn, writeTimeout time.Duration) transport.MultiConn {
	addr := transport.Iroh + "://" + conn.RemoteID().String()
	cctx = peer.CtxWithPeerAddr(cctx, addr)
	return &irohMultiConn{
		cctx:         cctx,
		conn:         conn,
		addr:         addr,
		writeTimeout: writeTimeout,
	}
}

type irohMultiConn struct {
	cctx         context.Context
	conn         *goiroh.Conn
	addr         string
	writeTimeout time.Duration
	bytesRead    atomic.Int64
	bytesWritten atomic.Int64
}

func (c *irohMultiConn) Context() context.Context {
	return c.cctx
}

// isConnDead reports whether err means the underlying QUIC connection is
// gone (idle timeout, peer close, endpoint shutdown) rather than a
// stream-level failure.
func (c *irohMultiConn) isConnDead(err error) bool {
	if err == nil {
		return false
	}
	if c.conn.Context().Err() != nil {
		return true
	}
	return errors.Is(err, net.ErrClosed) || errors.Is(err, goiroh.ErrEndpointClosed)
}

func (c *irohMultiConn) Accept() (conn net.Conn, err error) {
	stream, err := c.conn.AcceptStream(context.Background())
	if err != nil {
		if c.isConnDead(err) {
			err = transport.ErrConnClosed
		}
		return nil, err
	}
	return c.netConn(stream), nil
}

func (c *irohMultiConn) Open(ctx context.Context) (conn net.Conn, err error) {
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		if c.isConnDead(err) {
			return nil, transport.ErrConnClosed
		}
		return nil, err
	}
	return c.netConn(stream), nil
}

func (c *irohMultiConn) netConn(stream *goiroh.Stream) net.Conn {
	return irohNetConn{
		Stream:       stream,
		localAddr:    c.conn.LocalAddr(),
		remoteAddr:   c.conn.RemoteAddr(),
		writeTimeout: c.writeTimeout,
		bytesRead:    &c.bytesRead,
		bytesWritten: &c.bytesWritten,
	}
}

func (c *irohMultiConn) Addr() string {
	return c.addr
}

func (c *irohMultiConn) IsClosed() bool {
	select {
	case <-c.CloseChan():
		return true
	default:
		return false
	}
}

func (c *irohMultiConn) CloseChan() <-chan struct{} {
	return c.conn.Context().Done()
}

// Close sends the QUIC close frame; go-iroh's CloseWithError does not block.
func (c *irohMultiConn) Close() error {
	if err := c.conn.CloseWithError(closeCodeNormal, ""); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Debug("iroh conn closed with error", zap.Error(err))
	}
	return nil
}

func (c *irohMultiConn) BytesRead() int64 {
	return c.bytesRead.Load()
}

func (c *irohMultiConn) BytesWritten() int64 {
	return c.bytesWritten.Load()
}

type irohNetConn struct {
	*goiroh.Stream
	writeTimeout          time.Duration
	localAddr, remoteAddr net.Addr
	bytesRead             *atomic.Int64
	bytesWritten          *atomic.Int64
}

// Close closes the send side and cancels the receive side: a bidirectional
// QUIC stream stays readable after Close until the peer finishes or resets,
// which would keep sub-conn teardown waiting on a peer that never does.
func (c irohNetConn) Close() error {
	c.Stream.CancelRead(0)
	return c.Stream.Close()
}

func (c irohNetConn) Write(b []byte) (n int, err error) {
	if c.writeTimeout > 0 {
		if err = c.Stream.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			return
		}
	}
	n, err = c.Stream.Write(b)
	if n > 0 {
		c.bytesWritten.Add(int64(n))
	}
	return
}

func (c irohNetConn) Read(b []byte) (n int, err error) {
	n, err = c.Stream.Read(b)
	if n > 0 {
		c.bytesRead.Add(int64(n))
	}
	return
}

func (c irohNetConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c irohNetConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
