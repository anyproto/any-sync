package synctestproto

import "github.com/anyproto/protobuf/proto"

func (c *CounterIncrease) MsgSize() uint64 {
	if c != nil {
		return uint64(proto.Size(c))
	} else {
		return 0
	}
}
