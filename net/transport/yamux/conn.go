package yamux

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

func NewMultiConn(cctx context.Context, luConn *connutil.LastUsageConn, addr string, sess *yamux.Session) transport.MultiConn {
	cctx = peer.CtxWithPeerAddr(cctx, transport.Yamux+"://"+sess.RemoteAddr().String())
	return &yamuxConn{
		ctx:     cctx,
		luConn:  luConn,
		addr:    addr,
		Session: sess,
	}
}

type yamuxConn struct {
	ctx    context.Context
	luConn *connutil.LastUsageConn
	addr   string
	*yamux.Session
}

func (y *yamuxConn) Open(ctx context.Context) (conn net.Conn, err error) {
	if conn, err = y.Session.Open(); err != nil {
		return
	}
	return
}

func (y *yamuxConn) LastUsage() time.Time {
	return y.luConn.LastUsage()
}

func (y *yamuxConn) Context() context.Context {
	return y.ctx
}

func (y *yamuxConn) Addr() string {
	return transport.Yamux + "://" + y.addr
}

func (y *yamuxConn) Accept() (conn net.Conn, err error) {
	if conn, err = y.Session.Accept(); err != nil {
		if err == yamux.ErrSessionShutdown || err == io.EOF {
			err = transport.ErrConnClosed
		}
		return
	}
	return
}
