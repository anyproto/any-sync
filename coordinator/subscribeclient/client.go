package subscribeclient

import (
	"context"
	"errors"
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
)

const CName = "common.subscribeclient"

var log = logger.NewNamed(CName)

var (
	ErrSomeError = errors.New("some error")
)

func New() SubscribeClientService {
	return new(subscribeClient)
}

type EventCallback func(*coordinatorproto.NotifySubscribeEvent)

type SubscribeClientService interface {
	// `id` is just some id which is used to unsubscribe
	Subscribe(id string, eventType coordinatorproto.NotifyEventType, callback EventCallback)
	Unsubscribe(id string)
	app.ComponentRunnable
}

type callbacksMap map[string]EventCallback

type subscribeClient struct {
	mu        sync.Mutex
	callbacks map[coordinatorproto.NotifyEventType]callbacksMap
}

func (s *subscribeClient) Init(a *app.App) (err error) {
	return
}

func (s *subscribeClient) Name() (name string) {
	return CName
}

func (s *subscribeClient) Run(ctx context.Context) error {
	return nil
}

func (s *subscribeClient) Close(_ context.Context) (err error) {
	return nil
}

func (s *subscribeClient) Subscribe(id string, eventType coordinatorproto.NotifyEventType, callback EventCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.callbacks[eventType]
	if !ok {
		s.callbacks[eventType] = make(callbacksMap)
	}

	s.callbacks[eventType][id] = callback

}

func (s *subscribeClient) Unsubscribe(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, callbacks := range s.callbacks {
		delete(callbacks, id)
	}

}
