package synctest

import (
	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

type CounterRequest struct {
	peerId string
	*synctestproto.CounterRequest
}

func (c CounterRequest) Proto() (proto.Message, error) {
	return c.CounterRequest, nil
}

func NewCounterRequest(peerId, objectId string, counters []int32) CounterRequest {
	return CounterRequest{
		peerId: peerId,
		CounterRequest: &synctestproto.CounterRequest{
			ExistingValues: counters,
			ObjectId:       objectId,
		},
	}
}

func (c CounterRequest) PeerId() string {
	return c.peerId
}

func (c CounterRequest) ObjectId() string {
	return c.CounterRequest.ObjectId
}
