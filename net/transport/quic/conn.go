package quic

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

func newConn(cctx context.Context, qconn quic.Connection, writeTimeout time.Duration) transport.MultiConn {
	cctx = peer.CtxWithPeerAddr(cctx, transport.Quic+"://"+qconn.RemoteAddr().String())
	return &quicMultiConn{
		cctx:         cctx,
		Connection:   qconn,
		writeTimeout: writeTimeout,
	}
}

type quicMultiConn struct {
	cctx         context.Context
	writeTimeout time.Duration
	quic.Connection
}

func (q *quicMultiConn) Context() context.Context {
	return q.cctx
}

func (q *quicMultiConn) Accept() (conn net.Conn, err error) {
	stream, err := q.Connection.AcceptStream(context.Background())
	if err != nil {
		if errors.Is(err, quic.ErrServerClosed) {
			err = transport.ErrConnClosed
		} else if aerr, ok := err.(*quic.ApplicationError); ok && aerr.ErrorCode == 2 {
			err = transport.ErrConnClosed
		}
		return nil, err
	}
	return quicNetConn{
		Stream:       stream,
		localAddr:    q.LocalAddr(),
		remoteAddr:   q.RemoteAddr(),
		writeTimeout: q.writeTimeout,
	}, nil
}

func (q *quicMultiConn) Open(ctx context.Context) (conn net.Conn, err error) {
	stream, err := q.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return quicNetConn{
		Stream:     stream,
		localAddr:  q.LocalAddr(),
		remoteAddr: q.RemoteAddr(),
	}, nil
}

func (q *quicMultiConn) Addr() string {
	return transport.Quic + "://" + q.RemoteAddr().String()
}

func (q *quicMultiConn) IsClosed() bool {
	select {
	case <-q.CloseChan():
		return true
	default:
		return false
	}
}

func (q *quicMultiConn) CloseChan() <-chan struct{} {
	return q.Connection.Context().Done()
}

func (q *quicMultiConn) Close() error {
	return q.Connection.CloseWithError(2, "")
}

const (
	reset quic.StreamErrorCode = 0
)

type quicNetConn struct {
	quic.Stream
	writeTimeout          time.Duration
	localAddr, remoteAddr net.Addr
}

func (q quicNetConn) Close() error {
	// From quic docs: https://quic-go.net/docs/quic/streams/
	// "Calling Close on a quic.Stream closes the send side of the stream.
	//  Note that for bidirectional streams, Close only closes the send side of the stream.
	//  It is still possible to read from the stream until the peer closes or resets the stream."
	//
	// That's why we cancel read explicitly (same approach used in libp2p)
	q.Stream.CancelRead(reset)
	return q.Stream.Close()
}

func (q quicNetConn) Write(b []byte) (n int, err error) {
	if q.writeTimeout > 0 {
		if err = q.Stream.SetWriteDeadline(time.Now().Add(q.writeTimeout)); err != nil {
			return
		}
	}
	return q.Stream.Write(b)
}

func (q quicNetConn) LocalAddr() net.Addr {
	return q.localAddr
}

func (q quicNetConn) RemoteAddr() net.Addr {
	return q.remoteAddr
}
