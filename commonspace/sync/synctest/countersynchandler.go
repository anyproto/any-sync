package synctest

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/util/multiqueue"
)

type CounterSyncHandler struct {
	counter        *Counter
	requestHandler *CounterRequestHandler
	requestSender  *CounterRequestSender
	updateHandler  *CounterUpdateHandler
}

func (c *CounterSyncHandler) ApplyRequest(ctx context.Context, rq syncdeps.Request, requestSender syncdeps.RequestSender) error {
	collector := NewCounterResponseCollector(c.counter)
	return requestSender.SendRequest(ctx, rq, collector)
}

func NewCounterSyncHandler() syncdeps.SyncHandler {
	return &CounterSyncHandler{}
}

func (c *CounterSyncHandler) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (syncdeps.Request, error) {
	return c.updateHandler.HandleHeadUpdate(ctx, headUpdate)
}

func (c *CounterSyncHandler) TryAddMessage(ctx context.Context, id string, msg multiqueue.Sizeable, q *mb.MB[multiqueue.Sizeable]) error {
	return q.TryAdd(msg)
}

func (c *CounterSyncHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, updater syncdeps.QueueSizeUpdater, send func(resp proto.Message) error) (syncdeps.Request, error) {
	return c.requestHandler.HandleStreamRequest(ctx, rq, send)
}

func (c *CounterSyncHandler) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	return c.requestSender.SendStreamRequest(ctx, rq, receive)
}

func (c *CounterSyncHandler) Init(a *app.App) (err error) {
	peerProvider := a.MustComponent(PeerName).(*PeerProvider)
	c.counter = a.MustComponent(CounterName).(*Counter)
	c.requestHandler = &CounterRequestHandler{counter: c.counter}
	c.requestSender = &CounterRequestSender{peerProvider: a.MustComponent(PeerName).(*PeerProvider)}
	c.updateHandler = &CounterUpdateHandler{counter: c.counter, peerProvider: peerProvider}
	return nil
}

func (c *CounterSyncHandler) Name() (name string) {
	return syncdeps.CName
}

func (c *CounterSyncHandler) NewMessage() drpc.Message {
	return &CounterUpdate{}
}
