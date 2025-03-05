package streampool

import (
	"errors"
	"github.com/anyproto/any-sync/protobuf"
	drpc2 "github.com/planetscale/vtprotobuf/codec/drpc"
	"google.golang.org/protobuf/proto"
	"storj.io/drpc"
)

var (
	// EncodingProto drpc.Encoding implementation for gogo protobuf
	EncodingProto drpc.Encoding = protoEncoding{}
)

var (
	errNotAProtoMsg = errors.New("encoding: not a proto message")
)

type ProtoMessageGettable interface {
	ProtoMessage() (proto.Message, error)
}

type ProtoMessageSettable interface {
	ProtoMessageGettable
	SetProtoMessage(proto.Message) error
}

type protoEncoding struct{}

func (p protoEncoding) Marshal(msg drpc.Message) (res []byte, err error) {
	pmsg, ok := msg.(proto.Message)
	if !ok {
		if pmg, ok := msg.(ProtoMessageGettable); ok {
			pmsg, err = pmg.ProtoMessage()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errNotAProtoMsg
		}
	}
	return drpc2.Marshal(pmsg)
}

func (p protoEncoding) MarshalAppend(buf []byte, msg drpc.Message) (res []byte, err error) {
	pmsg, ok := msg.(proto.Message)
	if !ok {
		if pmg, ok := msg.(ProtoMessageGettable); ok {
			pmsg, err = pmg.ProtoMessage()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errNotAProtoMsg
		}
	}
	return protobuf.MarshalAppend(buf, pmsg)
}

func (p protoEncoding) Unmarshal(buf []byte, msg drpc.Message) (err error) {
	var pms ProtoMessageSettable
	pmsg, ok := msg.(proto.Message)
	if !ok {
		if pms, ok = msg.(ProtoMessageSettable); ok {
			pmsg, err = pms.ProtoMessage()
			if err != nil {
				return err
			}
		} else {
			return errNotAProtoMsg
		}
	}
	err = drpc2.Unmarshal(buf, pmsg)
	if err != nil {
		return err
	}
	if pms != nil {
		err = pms.SetProtoMessage(pmsg)
	}
	return
}
