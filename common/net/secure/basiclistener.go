package secure

import (
	"context"
	"net"
)

type basicListener struct {
	net.Listener
}

func newBasicListener(listener net.Listener) ContextListener {
	return &basicListener{listener}
}

func (b *basicListener) Accept(ctx context.Context) (context.Context, net.Conn, error) {
	conn, err := b.Listener.Accept()
	return ctx, conn, err
}
