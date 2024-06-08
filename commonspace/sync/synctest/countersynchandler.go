package synctest

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type CounterSyncHandler struct {
	requestHandler  *CounterRequestHandler
	requestSender   *CounterRequestSender
	responseHandler *CounterResponseHandler
	updateHandler   *CounterUpdateHandler
}

func NewCounterSyncHandler() syncdeps.SyncHandler {
	return &CounterSyncHandler{}
}

func (c *CounterSyncHandler) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (syncdeps.Request, error) {
	return c.updateHandler.HandleHeadUpdate(ctx, headUpdate)
}

func (c *CounterSyncHandler) TryAddMessage(ctx context.Context, msg drpc.Message, q *mb.MB[drpc.Message]) error {
	return q.TryAdd(msg)
}

func (c *CounterSyncHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, send func(resp proto.Message) error) (syncdeps.Request, error) {
	return c.requestHandler.HandleStreamRequest(ctx, rq, send)
}

func (c *CounterSyncHandler) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	return c.requestSender.SendStreamRequest(ctx, rq, receive)
}

func (c *CounterSyncHandler) HandleResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	return c.responseHandler.HandleResponse(ctx, peerId, objectId, resp)
}

func (c *CounterSyncHandler) NewResponse() syncdeps.Response {
	return c.responseHandler.NewResponse()
}

func (c *CounterSyncHandler) Init(a *app.App) (err error) {
	counter := a.MustComponent(CounterName).(*Counter)
	peerProvider := a.MustComponent(PeerName).(*PeerProvider)
	c.requestHandler = &CounterRequestHandler{counter: counter}
	c.requestSender = &CounterRequestSender{peerProvider: a.MustComponent(PeerName).(*PeerProvider)}
	c.responseHandler = &CounterResponseHandler{counter: counter}
	c.updateHandler = &CounterUpdateHandler{counter: counter, peerProvider: peerProvider}
	return nil
}

func (c *CounterSyncHandler) Name() (name string) {
	return syncdeps.CName
}

func (c *CounterSyncHandler) NewMessage() drpc.Message {
	return &CounterUpdate{}
}
