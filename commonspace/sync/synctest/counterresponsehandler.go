package synctest

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

type CounterResponseHandler struct {
	counter *Counter
}

func (c *CounterResponseHandler) NewResponse() syncdeps.Response {
	return &synctestproto.CounterIncrease{}
}

func (c *CounterResponseHandler) HandleResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	counterResp := resp.(*synctestproto.CounterIncrease)
	fmt.Println("handling value", peerId, objectId, counterResp.Value)
	c.counter.Add(counterResp.Value)
	return nil
}
