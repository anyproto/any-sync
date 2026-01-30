package encoding

import (
	"storj.io/drpc"

	"github.com/anyproto/any-sync/protobuf"
)

var (
	defaultProtoEncoding = protoEncoding{}
)

const maxBufSizeToReuse = 256 * 1024 // 256 KB

type protoEncoding struct{}

func (b protoEncoding) Marshal(msg drpc.Message) ([]byte, error) {
	return b.MarshalAppend(nil, msg)
}

func (b protoEncoding) MarshalAppend(buf []byte, msg drpc.Message) (res []byte, err error) {
	protoMessage, ok := msg.(protobuf.Message)
	if !ok {
		if protoMessageGettable, ok := msg.(ProtoMessageGettable); ok {
			protoMessage, err = protoMessageGettable.ProtoMessage()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, ErrNotAProtoMessage
		}
	}
	if cap(buf) > maxBufSizeToReuse {
		buf = make([]byte, 0, protoMessage.SizeVT())
	}
	return protobuf.MarshalAppend(buf, protoMessage)
}

func (b protoEncoding) Unmarshal(buf []byte, msg drpc.Message) (err error) {
	var protoMessageSettable ProtoMessageSettable
	protoMessage, ok := msg.(protobuf.Message)
	if !ok {
		if protoMessageSettable, ok = msg.(ProtoMessageSettable); ok {
			protoMessage, err = protoMessageSettable.ProtoMessage()
			if err != nil {
				return
			}
		} else {
			return ErrNotAProtoMessage
		}
	}
	err = protoMessage.UnmarshalVT(buf)
	if err != nil {
		return err
	}
	if protoMessageSettable != nil {
		err = protoMessageSettable.SetProtoMessage(protoMessage)
	}
	return
}
