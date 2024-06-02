package synctest

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/net/peer"
)

type CounterUpdateHandler struct {
	counter *Counter
}

func (c *CounterUpdateHandler) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (syncdeps.Request, error) {
	update := headUpdate.(CounterUpdate)
	c.counter.Add(update.counter)
	if c.counter.CheckComplete() {
		return nil, nil
	}
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	return NewCounterRequest(peerId, update.objectId, c.counter.KnownCounters()), nil
}
