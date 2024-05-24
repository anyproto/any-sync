package handshake

import (
	"github.com/anyproto/any-sync/net/internal/secureservice/handshake/handshakeproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type protoRes struct {
	protoType handshakeproto.ProtoType
	err       error
}

func newProtoChecker(types ...handshakeproto.ProtoType) ProtoChecker {
	return ProtoChecker{AllowedProtoTypes: types}
}
func TestIncomingProtoHandshake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		go func() {
			protoType, err := IncomingProtoHandshake(nil, c1, newProtoChecker(1))
			protoResCh <- protoRes{protoType: protoType, err: err}
		}()
		h := newHandshake()
		h.conn = c2

		// write desired proto
		require.NoError(t, h.writeProto(&handshakeproto.Proto{Proto: handshakeproto.ProtoType(1)}))
		msg, err := h.readMsg(msgTypeAck)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		res := <-protoResCh
		require.NoError(t, res.err)
		assert.Equal(t, handshakeproto.ProtoType(1), res.protoType)
	})
	t.Run("incompatible", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		go func() {
			protoType, err := IncomingProtoHandshake(nil, c1, newProtoChecker(1))
			protoResCh <- protoRes{protoType: protoType, err: err}
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
			err := OutgoingProtoHandshake(nil, c1, 1)
			protoResCh <- protoRes{err: err}
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
	t.Run("incompatible", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var protoResCh = make(chan protoRes, 1)
		go func() {
			err := OutgoingProtoHandshake(nil, c1, 1)
			protoResCh <- protoRes{err: err}
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
	c1, c2 := newConnPair(t)
	var (
		inResCh  = make(chan protoRes, 1)
		outResCh = make(chan protoRes, 1)
	)
	st := time.Now()
	go func() {
		err := OutgoingProtoHandshake(nil, c1, 0)
		outResCh <- protoRes{err: err}
	}()
	go func() {
		protoType, err := IncomingProtoHandshake(nil, c2, newProtoChecker(0, 1))
		inResCh <- protoRes{protoType: protoType, err: err}
	}()

	outRes := <-outResCh
	assert.NoError(t, outRes.err)

	inRes := <-inResCh
	assert.NoError(t, inRes.err)
	assert.Equal(t, handshakeproto.ProtoType(0), inRes.protoType)
	t.Log("dur", time.Since(st))
}
