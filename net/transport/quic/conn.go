package quic

import (
	"context"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/quic-go/quic-go"
	"net"
	"time"
)

func newConn(cctx context.Context, qconn quic.Connection) transport.MultiConn {
	return &quicMultiConn{qconn}
}

type quicMultiConn struct {
	quic.Connection
}

func (q *quicMultiConn) Context() context.Context {
	//TODO implement me
	panic("implement me")
}

func (q *quicMultiConn) Accept() (conn net.Conn, err error) {
	//TODO implement me
	panic("implement me")
}

func (q *quicMultiConn) Open(ctx context.Context) (conn net.Conn, err error) {
	//TODO implement me
	panic("implement me")
}

func (q *quicMultiConn) LastUsage() time.Time {
	//TODO implement me
	panic("implement me")
}

func (q *quicMultiConn) Addr() string {
	//TODO implement me
	panic("implement me")
}

func (q *quicMultiConn) IsClosed() bool {
	//TODO implement me
	panic("implement me")
}

func (q *quicMultiConn) Close() error {
	//TODO implement me
	panic("implement me")
}
