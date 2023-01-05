package secureservice

import (
	"context"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/timeoutconn"
	"github.com/libp2p/go-libp2p/core/crypto"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"net"
	"time"
)

type ContextListener interface {
	// Accept works like net.Listener accept but add context
	Accept(ctx context.Context) (context.Context, net.Conn, error)

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	Close() error

	// Addr returns the listener's network address.
	Addr() net.Addr
}

func newTLSListener(key crypto.PrivKey, lis net.Listener, timeoutMillis int) ContextListener {
	tr, _ := libp2ptls.New(key)
	return &tlsListener{
		tr:            tr,
		Listener:      lis,
		timeoutMillis: timeoutMillis,
	}
}

type tlsListener struct {
	net.Listener
	tr            *libp2ptls.Transport
	timeoutMillis int
}

func (p *tlsListener) Accept(ctx context.Context) (context.Context, net.Conn, error) {
	conn, err := p.Listener.Accept()
	if err != nil {
		return nil, nil, err
	}
	timeoutConn := timeoutconn.NewConn(conn, time.Duration(p.timeoutMillis)*time.Millisecond)
	return p.upgradeConn(ctx, timeoutConn)
}

func (p *tlsListener) upgradeConn(ctx context.Context, conn net.Conn) (context.Context, net.Conn, error) {
	secure, err := p.tr.SecureInbound(ctx, conn, "")
	if err != nil {
		return nil, nil, HandshakeError(err)
	}
	ctx = peer.CtxWithPeerId(ctx, secure.RemotePeer().String())
	return ctx, secure, nil
}
