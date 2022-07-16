package sync

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
)

type service struct {
}

const CName = "SyncService"

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	ch := make(chan *PubSubPayload)
	err = s.pubSub.Listen(ch)
	if err != nil {
		return
	}
	return nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) listen(ctx context.Context, ch chan *PubSubPayload) {
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-ch:
			// TODO: get object from object service and try to perform sync
		}
	}
}
