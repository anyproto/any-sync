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

var defaultSnappyEncoding = &snappyEncoding{}

// WrapConnEncoding wraps the drpc connection and replaces an encoding
func WrapConnEncoding(conn ConnUnblocked, useSnappy bool) ConnUnblocked {
	encConn := &connWrap{ConnUnblocked: conn}
	if useSnappy {
		encConn.snappyEnc = defaultSnappyEncoding
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

type streamWrap struct {
	drpc.Stream
	encoding drpc.Encoding
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
