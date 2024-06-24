package synctestproto

import "github.com/gogo/protobuf/proto"

func (c *CounterIncrease) MsgSize() uint64 {
	if c != nil {
		return uint64(proto.Size(c))
	} else {
		return 0
	}
}
