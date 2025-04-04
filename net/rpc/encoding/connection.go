package encoding

import (
	"context"
	"errors"

	"github.com/anyproto/protobuf/proto"
	"storj.io/drpc"
)

var ErrNotAProtoMessage = errors.New("encoding: not a proto message")

type ProtoMessageGettable interface {
	ProtoMessage() (proto.Message, error)
}

type ProtoMessageSettable interface {
	ProtoMessageGettable
	SetProtoMessage(proto.Message) error
}

// WrapConnEncoding wraps the drpc connection and replace an encoding
func WrapConnEncoding(conn drpc.Conn, useSnappy bool) drpc.Conn {
	encConn := &connWrap{Conn: conn}
	if useSnappy {
		encConn.snappyEnc = snappyPool.Get().(*snappyEncoding)
	}
	return encConn
}

type connWrap struct {
	drpc.Conn
	snappyEnc *snappyEncoding
}

func (c *connWrap) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) error {
	if c.snappyEnc != nil {
		enc = c.snappyEnc
	} else {
		enc = defaultProtoEncoding
	}
	return c.Conn.Invoke(ctx, rpc, enc, in, out)
}

func (c *connWrap) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (drpc.Stream, error) {
	if c.snappyEnc != nil {
		enc = c.snappyEnc
	} else {
		enc = defaultProtoEncoding
	}
	stream, err := c.Conn.NewStream(ctx, rpc, enc)
	if err != nil {
		return nil, err
	}
	return streamWrap{Stream: stream, encoding: enc}, nil
}

func (c *connWrap) Close() (err error) {
	err = c.Conn.Close()
	if c.snappyEnc != nil {
		snappyPool.Put(c.snappyEnc)
	}
	return
}

type streamWrap struct {
	drpc.Stream
	encoding     drpc.Encoding
	returnToPool bool
}

func (s streamWrap) MsgSend(msg drpc.Message, _ drpc.Encoding) error {
	return s.Stream.MsgSend(msg, s.encoding)
}

func (s streamWrap) MsgRecv(msg drpc.Message, _ drpc.Encoding) error {
	return s.Stream.MsgRecv(msg, s.encoding)
}

func (s streamWrap) CloseSend() (err error) {
	err = s.Stream.CloseSend()
	if s.returnToPool {
		snappyPool.Put(s.encoding.(*snappyEncoding))
	}
	return
}

func (s streamWrap) Close() (err error) {
	err = s.Stream.Close()
	if s.returnToPool {
		snappyPool.Put(s.encoding.(*snappyEncoding))
	}
	return
}
