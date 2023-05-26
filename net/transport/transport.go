package transport

import (
	"context"
	"net"
	"time"
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
	// LastUsage returns the time of the last connection activity
	LastUsage() time.Time
	// Addr returns remote peer address
	Addr() string
	// IsClosed returns true when connection is closed
	IsClosed() bool
	// Close closes the connection and all sub connections
	Close() error
}

type Accepter interface {
	Accept(mc MultiConn) (err error)
}
