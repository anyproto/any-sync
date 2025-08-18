package handshake

import (
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
