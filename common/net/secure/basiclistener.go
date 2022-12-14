package secure

import (
	"context"
	timeoutconn "github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/conn"
	"net"
	"time"
)

type basicListener struct {
	net.Listener
	timeoutMillis int
}

func newBasicListener(listener net.Listener, timeoutMillis int) ContextListener {
	return &basicListener{listener, timeoutMillis}
}

func (b *basicListener) Accept(ctx context.Context) (context.Context, net.Conn, error) {
	conn, err := b.Listener.Accept()
	if err != nil {
		return nil, nil, err
	}
	timeoutConn := timeoutconn.NewConn(conn, time.Duration(b.timeoutMillis)*time.Millisecond)
	return ctx, timeoutConn, err
}
