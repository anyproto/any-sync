package handshake

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
)

type protoRes struct {
	proto *handshakeproto.Proto
	err   error
}

func newProtoChecker(types ...handshakeproto.ProtoType) ProtoChecker {
	return ProtoChecker{AllowedProtoTypes: types}
}
func TestIncomingProtoHandshake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		go func() {
			proto, err := IncomingProtoHandshake(nil, c1, newProtoChecker(1))
			protoResCh <- protoRes{proto: proto, err: err}
		}()
		h := newHandshake()
		h.conn = c2

		// write desired proto
		require.NoError(t, h.writeProto(&handshakeproto.Proto{Proto: 1}))
		msg, err := h.readMsg(msgTypeAck)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		res := <-protoResCh
		require.NoError(t, res.err)
		assert.Equal(t, handshakeproto.ProtoType(1), res.proto.Proto)
	})
	t.Run("success encoding", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		var encodings = []handshakeproto.Encoding{handshakeproto.Encoding_Snappy, handshakeproto.Encoding_None}
		go func() {
			pt := newProtoChecker(1)
			pt.SupportedEncodings = encodings
			proto, err := IncomingProtoHandshake(nil, c1, pt)
			protoResCh <- protoRes{proto: proto, err: err}
		}()
		h := newHandshake()
		h.conn = c2

		// write desired proto
		require.NoError(t, h.writeProto(&handshakeproto.Proto{Proto: 1, Encodings: encodings}))
		msg, err := h.readMsg(msgTypeProto)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.ProtoType(1), msg.proto.Proto)
		assert.Equal(t, handshakeproto.Encoding_Snappy, msg.proto.Encodings[0])

		res := <-protoResCh
		require.NoError(t, res.err)
		assert.Equal(t, handshakeproto.ProtoType(1), res.proto.Proto)
		assert.Equal(t, handshakeproto.Encoding_Snappy, res.proto.Encodings[0])
	})
	t.Run("incompatible", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		go func() {
			proto, err := IncomingProtoHandshake(nil, c1, newProtoChecker(1))
			protoResCh <- protoRes{proto: proto, err: err}
		}()
		h := newHandshake()
		h.conn = c2

		// write desired proto
		require.NoError(t, h.writeProto(&handshakeproto.Proto{Proto: 0}))
		msg, err := h.readMsg(msgTypeAck)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.Error_IncompatibleProto, msg.ack.Error)
		res := <-protoResCh
		require.Error(t, res.err, ErrIncompatibleProto.Error())
	})
}

func TestOutgoingProtoHandshake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		go func() {
			proto, err := OutgoingProtoHandshake(nil, c1, &handshakeproto.Proto{Proto: 1})
			protoResCh <- protoRes{err: err, proto: proto}
		}()
		h := newHandshake()
		h.conn = c2

		msg, err := h.readMsg(msgTypeProto)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.ProtoType(1), msg.proto.Proto)
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))

		res := <-protoResCh
		assert.NoError(t, res.err)
	})
	t.Run("success encoding", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		var encodings = []handshakeproto.Encoding{handshakeproto.Encoding_Snappy, handshakeproto.Encoding_None}
		go func() {
			proto, err := OutgoingProtoHandshake(nil, c1, &handshakeproto.Proto{Proto: 1, Encodings: encodings})
			protoResCh <- protoRes{err: err, proto: proto}
		}()
		h := newHandshake()
		h.conn = c2

		msg, err := h.readMsg(msgTypeProto)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.ProtoType(1), msg.proto.Proto)
		assert.Equal(t, handshakeproto.Encoding_Snappy, msg.proto.Encodings[0])
		require.NoError(t, h.writeProto(msg.proto))

		res := <-protoResCh
		assert.Equal(t, handshakeproto.ProtoType(1), res.proto.Proto)
		assert.Equal(t, handshakeproto.Encoding_Snappy, res.proto.Encodings[0])
		assert.NoError(t, res.err)
	})
	t.Run("incompatible", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		go func() {
			proto, err := OutgoingProtoHandshake(nil, c1, &handshakeproto.Proto{Proto: 1})
			protoResCh <- protoRes{err: err, proto: proto}
		}()
		h := newHandshake()
		h.conn = c2

		msg, err := h.readMsg(msgTypeProto)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.ProtoType(1), msg.proto.Proto)
		require.NoError(t, h.writeAck(handshakeproto.Error_IncompatibleProto))

		res := <-protoResCh
		assert.EqualError(t, res.err, ErrRemoteIncompatibleProto.Error())
	})
}

func TestEndToEndProto(t *testing.T) {
	t.Run("no encoding", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var (
			inResCh  = make(chan protoRes, 1)
			outResCh = make(chan protoRes, 1)
		)
		st := time.Now()
		go func() {
			proto, err := OutgoingProtoHandshake(nil, c1, &handshakeproto.Proto{Proto: 0})
			outResCh <- protoRes{err: err, proto: proto}
		}()
		go func() {
			proto, err := IncomingProtoHandshake(nil, c2, newProtoChecker(0, 1))
			inResCh <- protoRes{proto: proto, err: err}
		}()

		outRes := <-outResCh
		assert.NoError(t, outRes.err)

		inRes := <-inResCh
		assert.NoError(t, inRes.err)
		assert.Equal(t, handshakeproto.ProtoType(0), inRes.proto.Proto)
		t.Log("dur", time.Since(st))
	})
	t.Run("encoding", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var (
			inResCh   = make(chan protoRes, 1)
			outResCh  = make(chan protoRes, 1)
			encodings = []handshakeproto.Encoding{handshakeproto.Encoding_Snappy, handshakeproto.Encoding_None}
		)
		st := time.Now()
		go func() {
			proto, err := OutgoingProtoHandshake(nil, c1, &handshakeproto.Proto{Proto: 0, Encodings: encodings})
			outResCh <- protoRes{err: err, proto: proto}
		}()
		go func() {
			pt := newProtoChecker(0, 1)
			pt.SupportedEncodings = encodings
			proto, err := IncomingProtoHandshake(nil, c2, pt)
			inResCh <- protoRes{proto: proto, err: err}
		}()

		outRes := <-outResCh
		assert.NoError(t, outRes.err)
		assert.Equal(t, handshakeproto.ProtoType(0), outRes.proto.Proto)
		assert.Equal(t, handshakeproto.Encoding_Snappy, outRes.proto.Encodings[0])

		inRes := <-inResCh
		assert.NoError(t, inRes.err)
		assert.Equal(t, handshakeproto.ProtoType(0), inRes.proto.Proto)
		assert.Equal(t, handshakeproto.Encoding_Snappy, inRes.proto.Encodings[0])

		t.Log("dur", time.Since(st))
	})
}

// blockingCloseConn models a stream whose Close blocks on the transport
type blockingCloseConn struct {
	net.Conn
	closeCalled chan struct{}
	release     chan struct{}
	once        sync.Once
}

func (c *blockingCloseConn) Close() error {
	c.once.Do(func() { close(c.closeCalled) })
	<-c.release
	return c.Conn.Close()
}

func TestOutgoingProtoHandshake_CancelDoesNotWaitForClose(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	conn := &blockingCloseConn{Conn: c1, closeCalled: make(chan struct{}), release: make(chan struct{})}
	defer close(conn.release)
	// the remote reads the proto but never answers
	go func() { _, _ = io.Copy(io.Discard, c2) }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := OutgoingProtoHandshake(ctx, conn, &handshakeproto.Proto{Proto: 1})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second, "the caller must not wait on the blocked close")

	// the conn is still closed, off the caller's path
	select {
	case <-conn.closeCalled:
	case <-time.After(time.Second):
		t.Fatal("abandoned handshake conn was not closed")
	}
}

// noDeadlineConn models a conn whose SetDeadline is a no-op (as on wasm)
type noDeadlineConn struct {
	net.Conn
}

func (noDeadlineConn) SetDeadline(time.Time) error { return nil }

func TestOutgoingProtoHandshakeWithCloser_Cancel(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()
	conn := noDeadlineConn{Conn: c1}

	// the remote reads the proto, then never answers
	remoteRead := make(chan error, 1)
	go func() {
		h := newHandshake()
		h.conn = c2
		if _, err := h.readMsg(msgTypeProto); err != nil {
			remoteRead <- err
			return
		}
		// whatever comes next must be the close, never an ack
		_, err := c2.Read(make([]byte, 1))
		remoteRead <- err
	}()

	closed := make(chan net.Conn, 1)
	closer := func(c net.Conn) {
		closed <- c
		_ = c.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := OutgoingProtoHandshakeWithCloser(ctx, conn, &handshakeproto.Proto{Proto: 1}, closer)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// a conn without deadline support is still closed, through the closer
	select {
	case c := <-closed:
		assert.Equal(t, net.Conn(conn), c)
	case <-time.After(time.Second):
		t.Fatal("conn was not handed to the closer")
	}
	select {
	case err = <-remoteRead:
		assert.ErrorIs(t, err, io.EOF, "an abandoned handshake must not write an ack")
	case <-time.After(time.Second):
		t.Fatal("remote did not observe the close")
	}
}

func TestOutgoingProtoHandshakeWithCloser_IOErrorUsesCloser(t *testing.T) {
	c1, c2 := net.Pipe()
	// the remote goes away mid-handshake
	go func() {
		h := newHandshake()
		h.conn = c2
		_, _ = h.readMsg(msgTypeProto)
		_ = c2.Close()
	}()
	closed := make(chan struct{}, 1)
	closer := func(c net.Conn) {
		closed <- struct{}{}
		_ = c.Close()
	}
	_, err := OutgoingProtoHandshakeWithCloser(context.Background(), c1, &handshakeproto.Proto{Proto: 1}, closer)
	require.Error(t, err)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("I/O error close did not go through the closer")
	}
}

func TestHandshakeError_Unwrap(t *testing.T) {
	err := error(HandshakeError{Err: io.EOF})
	assert.ErrorIs(t, err, io.EOF)
	assert.ErrorIs(t, HandshakeError{Err: os.ErrDeadlineExceeded}, os.ErrDeadlineExceeded)
	// protocol-level sentinels keep matching by value and wrap nothing
	assert.ErrorIs(t, ErrIncompatibleVersion, ErrIncompatibleVersion)
	assert.NotErrorIs(t, ErrIncompatibleProto, ErrIncompatibleVersion)
	assert.Nil(t, errors.Unwrap(ErrIncompatibleVersion))
	// a wrapped transport error is not mistaken for a protocol sentinel
	assert.NotErrorIs(t, HandshakeError{Err: io.EOF}, ErrIncompatibleVersion)
}
