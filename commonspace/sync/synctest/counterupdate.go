package synctest

import (
	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

type CounterUpdate struct {
	counter  int32
	objectId string
}

func (c *CounterUpdate) message() proto.Message {
	return &synctestproto.CounterIncrease{
		Value:    c.counter,
		ObjectId: c.objectId,
	}
}

func (c *CounterUpdate) SetProtoMessage(message proto.Message) error {
	msg := message.(*synctestproto.CounterIncrease)
	c.counter = msg.Value
	c.objectId = msg.ObjectId
	return nil
}

func (c *CounterUpdate) ProtoMessage() (proto.Message, error) {
	if c.objectId == "" {
		return &synctestproto.CounterIncrease{}, nil
	}
	return c.message(), nil
}
