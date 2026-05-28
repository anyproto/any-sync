//go:build !js

package webtransport

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	wt "github.com/quic-go/webtransport-go"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

// session is an interface over *webtransport.Session for testability.
type session interface {
	OpenStreamSync(context.Context) (*wt.Stream, error)
	AcceptStream(context.Context) (*wt.Stream, error)
	Context() context.Context
	CloseWithError(wt.SessionErrorCode, string) error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// wtNetConn wraps a *webtransport.Stream as a net.Conn.
type wtNetConn struct {
	*wt.Stream
	writeTimeout          time.Duration
	localAddr, remoteAddr net.Addr
	bytesRead             *atomic.Int64
	bytesWritten          *atomic.Int64
}

func (c wtNetConn) Close() error {
	c.Stream.CancelRead(0)
	return c.Stream.Close()
}

func (c wtNetConn) Write(b []byte) (n int, err error) {
	if c.writeTimeout > 0 {
		if err = c.Stream.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			return
		}
	}
	n, err = c.Stream.Write(b)
	if n > 0 && c.bytesWritten != nil {
		c.bytesWritten.Add(int64(n))
	}
	return
}

func (c wtNetConn) Read(b []byte) (n int, err error) {
	n, err = c.Stream.Read(b)
	if n > 0 && c.bytesRead != nil {
		c.bytesRead.Add(int64(n))
	}
	return
}

func (c wtNetConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c wtNetConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func newConn(cctx context.Context, sess session, remoteAddr string, writeTimeout, closeTimeout time.Duration) transport.MultiConn {
	cctx = peer.CtxWithPeerAddr(cctx, transport.WebTransport+"://"+remoteAddr)
	return &wtMultiConn{
		cctx:         cctx,
		session:      sess,
		remoteAddr:   remoteAddr,
		writeTimeout: writeTimeout,
		closeTimeout: closeTimeout,
	}
}

type wtMultiConn struct {
	cctx         context.Context
	session      session
	remoteAddr   string
	writeTimeout time.Duration
	closeTimeout time.Duration
	bytesRead    atomic.Int64
	bytesWritten atomic.Int64
}

func (m *wtMultiConn) BytesRead() int64 {
	return m.bytesRead.Load()
}

func (m *wtMultiConn) BytesWritten() int64 {
	return m.bytesWritten.Load()
}

func (m *wtMultiConn) Context() context.Context {
	return m.cctx
}

func (m *wtMultiConn) Accept() (conn net.Conn, err error) {
	stream, err := m.session.AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}
	return wtNetConn{
		Stream:       stream,
		localAddr:    m.session.LocalAddr(),
		remoteAddr:   m.session.RemoteAddr(),
		writeTimeout: m.writeTimeout,
		bytesRead:    &m.bytesRead,
		bytesWritten: &m.bytesWritten,
	}, nil
}

func (m *wtMultiConn) Open(ctx context.Context) (conn net.Conn, err error) {
	stream, err := m.session.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return wtNetConn{
		Stream:       stream,
		localAddr:    m.session.LocalAddr(),
		remoteAddr:   m.session.RemoteAddr(),
		writeTimeout: m.writeTimeout,
		bytesRead:    &m.bytesRead,
		bytesWritten: &m.bytesWritten,
	}, nil
}

func (m *wtMultiConn) Addr() string {
	return transport.WebTransport + "://" + m.remoteAddr
}

func (m *wtMultiConn) IsClosed() bool {
	select {
	case <-m.CloseChan():
		return true
	default:
		return false
	}
}

func (m *wtMultiConn) CloseChan() <-chan struct{} {
	return m.session.Context().Done()
}

func (m *wtMultiConn) Close() error {
	closeWait := make(chan struct{})
	go func() {
		defer close(closeWait)
		_ = m.session.CloseWithError(0, "")
	}()
	select {
	case <-closeWait:
	case <-time.After(m.closeTimeout):
		log.Warn("webtransport session close timed out", zap.String("addr", m.remoteAddr))
	}
	return nil
}
