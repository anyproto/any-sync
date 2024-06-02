package synctest

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type CounterSyncDepsFactory struct {
	syncDeps syncdeps.SyncDeps
}

func NewCounterSyncDepsFactory() syncdeps.SyncDepsFactory {
	return &CounterSyncDepsFactory{}
}

func (c *CounterSyncDepsFactory) Init(a *app.App) (err error) {
	counter := a.MustComponent(CounterName).(*Counter)
	requestHandler := &CounterRequestHandler{counter: counter}
	requestSender := &CounterRequestSender{peerProvider: a.MustComponent(PeerName).(*PeerProvider)}
	responseHandler := &CounterResponseHandler{counter: counter}
	updateHandler := &CounterUpdateHandler{counter: counter}
	c.syncDeps = syncdeps.SyncDeps{
		HeadUpdateHandler: updateHandler,
		ResponseHandler:   responseHandler,
		RequestHandler:    requestHandler,
		RequestSender:     requestSender,
		MergeFilter: func(ctx context.Context, msg drpc.Message, q *mb.MB[drpc.Message]) error {
			return nil
		},
		ReadMessageConstructor: func() drpc.Message {
			return &CounterUpdate{}
		},
	}
	return nil
}

func (c *CounterSyncDepsFactory) Name() (name string) {
	return syncdeps.CName
}

func (c *CounterSyncDepsFactory) SyncDeps() syncdeps.SyncDeps {
	return c.syncDeps
}
