package synctest

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

type CounterResponseCollector struct {
	counter *Counter
}

func NewCounterResponseCollector(counter *Counter) *CounterResponseCollector {
	return &CounterResponseCollector{counter: counter}
}

func (c *CounterResponseCollector) CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	counterResp := resp.(*synctestproto.CounterIncrease)
	c.counter.Add(counterResp.Value)
	return nil
}
