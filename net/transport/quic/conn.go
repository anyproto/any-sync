//go:generate mockgen -package=mock_quic -destination=mock_quic/mock_packet_conn.go net PacketConn
//go:generate mockgen -package=mock_quic -source=$GOFILE -destination=mock_quic/mock_quic_conn.go connection
package quic

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

type connection interface {
	OpenStreamSync(context.Context) (*quic.Stream, error)
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	CloseWithError(quic.ApplicationErrorCode, string) error
	Context() context.Context
	AcceptStream(context.Context) (*quic.Stream, error)
}

func newConn(cctx context.Context, udpConn net.PacketConn, qconn connection, closeTimeout, writeTimeout time.Duration) transport.MultiConn {
	cctx = peer.CtxWithPeerAddr(cctx, transport.Quic+"://"+qconn.RemoteAddr().String())
	return &quicMultiConn{
		cctx:         cctx,
		udpConn:      udpConn,
		connection:   qconn,
		writeTimeout: writeTimeout,
		closeTimeout: closeTimeout,
	}
}

type quicMultiConn struct {
	udpConn      net.PacketConn
	cctx         context.Context
	writeTimeout time.Duration
	closeTimeout time.Duration
	bytesRead    atomic.Int64
	bytesWritten atomic.Int64
	connection
}

func (q *quicMultiConn) BytesRead() int64 {
	return q.bytesRead.Load()
}

func (q *quicMultiConn) BytesWritten() int64 {
	return q.bytesWritten.Load()
}

func (q *quicMultiConn) Context() context.Context {
	return q.cctx
}

// isConnDead reports whether err means the underlying QUIC connection is gone
// (idle timeout, peer-initiated close, or an already-closed connection), as
// opposed to a transient or stream-level error.
func isConnDead(err error) bool {
	if err == nil {
		return false
	}
	var idle *quic.IdleTimeoutError
	if errors.As(err, &idle) {
		return true
	}
	var appErr *quic.ApplicationError
	if errors.As(err, &appErr) && appErr.ErrorCode == 2 {
		return true
	}
	return errors.Is(err, quic.ErrServerClosed) || errors.Is(err, net.ErrClosed)
}

func (q *quicMultiConn) Accept() (conn net.Conn, err error) {
	stream, err := q.connection.AcceptStream(context.Background())
	if err != nil {
		if isConnDead(err) {
			err = transport.ErrConnClosed
		}
		return nil, err
	}
	return quicNetConn{
		Stream:       stream,
		localAddr:    q.LocalAddr(),
		remoteAddr:   q.RemoteAddr(),
		writeTimeout: q.writeTimeout,
		bytesRead:    &q.bytesRead,
		bytesWritten: &q.bytesWritten,
	}, nil
}

func (q *quicMultiConn) Open(ctx context.Context) (conn net.Conn, err error) {
	stream, err := q.OpenStreamSync(ctx)
	if err != nil {
		if isConnDead(err) {
			return nil, transport.ErrConnClosed
		}
		return nil, err
	}
	return quicNetConn{
		Stream:       stream,
		localAddr:    q.LocalAddr(),
		remoteAddr:   q.RemoteAddr(),
		writeTimeout: q.writeTimeout,
		bytesRead:    &q.bytesRead,
		bytesWritten: &q.bytesWritten,
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
	return q.connection.Context().Done()
}

func (q *quicMultiConn) Close() error {
	// if we have the udp connection saved in quicMultiConn, then we manage it ourselves
	// we don't manage/close the udp connections when we accept streams, otherwise we would kill the server
	closeWait := make(chan struct{})
	var isTimeout atomic.Bool
	go func() {
		select {
		case <-closeWait:
		case <-time.After(q.closeTimeout):
			isTimeout.Store(true)
			if q.udpConn != nil {
				err := q.udpConn.Close()
				if err != nil && !errors.Is(err, net.ErrClosed) {
					log.Error("udp conn closed with error", zap.Error(err))
				}
			}
		}
	}()
	go func() {
		err := q.connection.CloseWithError(2, "")
		if err != nil {
			log.Error("quic conn closed with error", zap.Error(err))
		}
		if !isTimeout.Load() && q.udpConn != nil {
			err := q.udpConn.Close()
			if err != nil && !errors.Is(err, net.ErrClosed) {
				log.Error("udp conn closed with error", zap.Error(err))
			}
		}
		close(closeWait)
	}()
	return nil
}

const (
	reset quic.StreamErrorCode = 0
)

type quicNetConn struct {
	*quic.Stream
	writeTimeout          time.Duration
	localAddr, remoteAddr net.Addr
	bytesRead             *atomic.Int64
	bytesWritten          *atomic.Int64
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
	n, err = q.Stream.Write(b)
	if n > 0 && q.bytesWritten != nil {
		q.bytesWritten.Add(int64(n))
	}
	return
}

func (q quicNetConn) Read(b []byte) (n int, err error) {
	n, err = q.Stream.Read(b)
	if n > 0 && q.bytesRead != nil {
		q.bytesRead.Add(int64(n))
	}
	return
}

func (q quicNetConn) LocalAddr() net.Addr {
	return q.localAddr
}

func (q quicNetConn) RemoteAddr() net.Addr {
	return q.remoteAddr
}

