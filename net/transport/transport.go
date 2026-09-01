//go:generate mockgen -destination mock_transport/mock_transport.go github.com/anyproto/any-sync/net/transport Transport,MultiConn
package transport

import (
	"context"
	"net"
	"time"
)

var (
	// ErrConnClosed is returned by transport Open/Accept when the underlying
	// connection is gone (idle timeout, peer close, server close). It unwraps to
	// net.ErrClosed so callers can match either sentinel.
	ErrConnClosed error = connClosedError{}
)

// connClosedError carries a distinctive message (to tell it apart from other
// connection errors in logs) while unwrapping to net.ErrClosed.
type connClosedError struct{}

func (connClosedError) Error() string { return "transport connection closed" }

func (connClosedError) Unwrap() error { return net.ErrClosed }

const (
	Yamux        = "yamux"
	Quic         = "quic"
	WebTransport = "webtransport"
	Iroh         = "iroh"

	// IrohCName names the iroh transport component; peerservice looks it up
	// by name so binaries that never register it do not link go-iroh.
	IrohCName = "net.transport.iroh"
)

// Transport is a common interface for a network transport
type Transport interface {
	// SetAccepter sets accepter that will be called for new connections
	// this method should be called before app start
	SetAccepter(accepter Accepter)
	// Dial creates a new connection by given address
	Dial(ctx context.Context, addr string) (mc MultiConn, err error)
}

// MultiConn is an object of multiplexing connection containing handshake info
type MultiConn interface {
	// Context returns the connection context that contains handshake details
	Context() context.Context
	// Accept accepts new sub connections
	Accept() (conn net.Conn, err error)
	// Open opens new sub connection
	Open(ctx context.Context) (conn net.Conn, err error)
	// Addr returns remote peer address
	Addr() string
	// IsClosed returns true when connection is closed
	IsClosed() bool
	// CloseChan returns a channel that will be closed with connection
	CloseChan() <-chan struct{}
	// Close closes the connection and all sub connections
	Close() error
	// BytesRead returns the cumulative number of bytes received from the peer
	// at the transport boundary (post-decryption, post-compression). For yamux
	// this includes session framing on top of the underlying TCP conn; for
	// QUIC/webtransport this counts only stream-level bytes and excludes
	// QUIC/UDP framing overhead.
	BytesRead() int64
	// BytesWritten returns the cumulative number of bytes sent to the peer
	// at the transport boundary (see BytesRead for level-of-detail caveats).
	BytesWritten() int64
}

type Accepter interface {
	Accept(mc MultiConn) (err error)
}

// ConnCloseKind classifies how an outbound connection ended, for transport
// quality tracking.
type ConnCloseKind int

const (
	// ConnCloseNeutral carries no signal about path quality (local close,
	// graceful remote close, short-lived non-timeout errors).
	ConnCloseNeutral ConnCloseKind = iota
	// ConnCloseDegraded means the path went black under an established
	// connection: keepalives stopped being answered shortly after the
	// handshake succeeded (the DPI-degradation signature).
	ConnCloseDegraded
	// ConnCloseHealthy means the connection lived long enough to prove the
	// path works, whatever finally closed it.
	ConnCloseHealthy
)

// ConnCloseEvent describes the end of an outbound (dialed) connection.
type ConnCloseEvent struct {
	PeerId       string
	Kind         ConnCloseKind
	Lifetime     time.Duration
	BytesRead    int64
	BytesWritten int64
	Cause        error
}

// ConnObserverSetter is implemented by transports able to report close events
// of the connections they dialed. The observer must not block: it is called
// from per-connection watcher goroutines.
type ConnObserverSetter interface {
	SetConnObserver(observer func(ev ConnCloseEvent))
}
