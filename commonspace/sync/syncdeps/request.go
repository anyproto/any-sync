package syncdeps

import "github.com/anyproto/protobuf/proto"

type Request interface {
	PeerId() string
	ObjectId() string
	Proto() (proto.Message, error)
	MsgSize() uint64
}
