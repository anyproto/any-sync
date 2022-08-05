package rpc

import (
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"
)

var Encoding = enc{}

type enc struct{}

func (e enc) Marshal(msg drpc.Message) ([]byte, error) {
	return msg.(proto.Marshaler).Marshal()
}

func (e enc) Unmarshal(buf []byte, msg drpc.Message) error {
	return msg.(proto.Unmarshaler).Unmarshal(buf)
}
