package encoding

import (
	"context"
	"errors"

	"storj.io/drpc"
	"storj.io/drpc/drpcwire"

	"github.com/anyproto/any-sync/protobuf"
)

var ErrNotAProtoMessage = errors.New("encoding: not a proto message")

type ProtoMessageGettable interface {
	ProtoMessage() (protobuf.Message, error)
}

type ProtoMessageSettable interface {
	ProtoMessageGettable
	SetProtoMessage(protobuf.Message) error
}

// WrapConnEncoding wraps the drpc connection and replace an encoding
func WrapConnEncoding(conn ConnUnblocked, useSnappy bool) ConnUnblocked {
	encConn := &connWrap{ConnUnblocked: conn}
	if useSnappy {
		encConn.snappyEnc = snappyPool.Get().(*snappyEncoding)
	}
	return encConn
}

type connWrap struct {
	ConnUnblocked
	snappyEnc *snappyEncoding
}

type ConnUnblocked interface {
	drpc.Conn
	Unblocked() <-chan struct{}
}

func (c *connWrap) Unblocked() <-chan struct{} {
	return c.ConnUnblocked.Unblocked()
}

func (c *connWrap) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) error {
	if c.snappyEnc != nil {
		enc = c.snappyEnc
	} else {
		enc = defaultProtoEncoding
	}
	return c.ConnUnblocked.Invoke(ctx, rpc, enc, in, out)
}

func (c *connWrap) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (drpc.Stream, error) {
	if c.snappyEnc != nil {
		enc = c.snappyEnc
	} else {
		enc = defaultProtoEncoding
	}
	stream, err := c.ConnUnblocked.NewStream(ctx, rpc, enc)
	if err != nil {
		return nil, err
	}
	return streamWrap{Stream: stream, encoding: enc}, nil
}

func (c *connWrap) Close() (err error) {
	err = c.ConnUnblocked.Close()
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

type streamRawWrite interface {
	RawWrite(kind drpcwire.Kind, data []byte) (err error)
}

func (s streamWrap) MsgSend(msg drpc.Message, _ drpc.Encoding) error {
	return s.Stream.MsgSend(msg, s.encoding)
}

func (s streamWrap) MsgRecv(msg drpc.Message, _ drpc.Encoding) error {
	return s.Stream.MsgRecv(msg, s.encoding)
}

func (s streamWrap) RawWrite(kind drpcwire.Kind, data []byte) (err error) {
	return s.Stream.(streamRawWrite).RawWrite(kind, data)
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
