package syncdeps

import "github.com/gogo/protobuf/proto"

type Request interface {
	PeerId() string
	ObjectId() string
	Proto() (proto.Message, error)
}
