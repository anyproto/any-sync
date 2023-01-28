package streampool

import (
	"errors"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"
)

var (
	// EncodingProto drpc.Encoding implementation for gogo protobuf
	EncodingProto drpc.Encoding = protoEncoding{}
)

var (
	errNotAProtoMsg = errors.New("encoding: not a proto message")
)

type protoEncoding struct{}

func (p protoEncoding) Marshal(msg drpc.Message) ([]byte, error) {
	pmsg, ok := msg.(proto.Message)
	if !ok {
		return nil, errNotAProtoMsg
	}
	return proto.Marshal(pmsg)
}

func (p protoEncoding) Unmarshal(buf []byte, msg drpc.Message) error {
	pmsg, ok := msg.(proto.Message)
	if !ok {
		return errNotAProtoMsg
	}
	return proto.Unmarshal(buf, pmsg)
}
