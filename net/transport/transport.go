//go:generate mockgen -destination mock_transport/mock_transport.go github.com/anyproto/any-sync/net/transport Transport,MultiConn
package transport

import (
	"context"
	"errors"
	"net"
)

var (
	ErrConnClosed = errors.New("connection closed")
)

const (
	Yamux = "yamux"
	Quic  = "quic"
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
}

type Accepter interface {
	Accept(mc MultiConn) (err error)
}
