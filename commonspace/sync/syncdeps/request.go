package syncdeps

import "google.golang.org/protobuf/proto"

type Request interface {
	PeerId() string
	ObjectId() string
	Proto() (proto.Message, error)
	MsgSize() uint64
}
