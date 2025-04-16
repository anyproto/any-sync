package syncdeps

import "github.com/anyproto/any-sync/commonspace/spacesyncproto"

type Message interface {
	ObjectId() string
	MsgSize() uint64
	ObjectType() spacesyncproto.ObjectType
}
