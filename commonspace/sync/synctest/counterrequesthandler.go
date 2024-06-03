package synctest

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

type CounterRequestHandler struct {
	counter *Counter
}

func (c *CounterRequestHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, send func(resp proto.Message) error) (syncdeps.Request, error) {
	counterRequest := rq.(CounterRequest)
	toSend, toAsk := c.counter.DiffCurrentNew(counterRequest.ExistingValues)
	slices.Sort(toSend)
	for _, value := range toSend {
		_ = send(&synctestproto.CounterIncrease{
			Value:    value,
			ObjectId: counterRequest.ObjectId(),
		})
	}
	if len(toAsk) == 0 {
		return nil, nil
	}
	return NewCounterRequest(counterRequest.PeerId(), counterRequest.ObjectId(), c.counter.Dump()), nil
}
