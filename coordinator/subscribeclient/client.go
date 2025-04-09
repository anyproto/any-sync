package subscribeclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
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
	Subscribe(eventType coordinatorproto.NotifyEventType, callback EventCallback) error
	app.ComponentRunnable
}

type subscribeClient struct {
	nodeconf nodeconf.Service
	pool     pool.Pool

	mucb      sync.Mutex
	callbacks map[coordinatorproto.NotifyEventType]EventCallback

	mu     sync.Mutex
	stream *stream
	close  chan struct{}
}

func (s *subscribeClient) Init(a *app.App) (err error) {
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.close = make(chan struct{})
	s.callbacks = make(map[coordinatorproto.NotifyEventType]EventCallback)
	return
}

func (s *subscribeClient) Name() (name string) {
	return CName
}

func (s *subscribeClient) Run(ctx context.Context) error {
	return nil
}

func (s *subscribeClient) Close(_ context.Context) (err error) {
	s.mu.Lock()
	if s.stream != nil {
		_ = s.stream.Close()
	}
	s.mu.Unlock()
	select {
	case <-s.close:
	default:
		close(s.close)
	}
	return nil
}

func (s *subscribeClient) Subscribe(eventType coordinatorproto.NotifyEventType, callback EventCallback) error {
	s.mucb.Lock()
	defer s.mucb.Unlock()

	_, ok := s.callbacks[eventType]
	if ok {
		return fmt.Errorf("event type %s is already registered", eventType.String())
	}

	s.callbacks[eventType] = callback
	return nil
}

func (s *subscribeClient) openStream(ctx context.Context) (st *stream, err error) {
	log.Warn("streamWatcher: trying to connect")
	pr, err := s.pool.GetOneOf(ctx, s.nodeconf.CoordinatorPeers())
	if err != nil {
		log.Warn("streamWatcher: pool error", zap.Error(err))
		return nil, err
	}
	pr.SetTTL(time.Hour * 24)
	dc, err := pr.AcquireDrpcConn(ctx)
	if err != nil {
		log.Warn("streamWatcher: drpc conn error")
		return nil, err
	}
	req := &coordinatorproto.NotifySubscribeRequest{
		EventType: coordinatorproto.NotifyEventType_InboxNewMessageEvent,
	}
	rpcStream, err := coordinatorproto.NewDRPCCoordinatorClient(dc).NotifySubscribe(ctx, req)
	if err != nil {
		log.Warn("streamWatcher: notify subscribe error")
		return nil, rpcerr.Unwrap(err)
	}
	return runStream(rpcStream), nil
}

func (s *subscribeClient) streamWatcher() {
	var (
		err error
		st  *stream
		i   int
	)
	for {
		log.Info("streamWatcher: open inbox stream")
		if st, err = s.openStream(context.Background()); err != nil {
			// can't open stream, we will retry until success connection or close
			log.Error("streamWatcher: watch inbox error, retry", zap.Error(err))
			if i < 10 {
				i++
			}
			sleepTime := time.Second * time.Duration(i)
			log.Error("streamWatcher: watch inbox erro, retry", zap.Error(err), zap.Duration("waitTime", sleepTime))
			select {
			case <-time.After(sleepTime):
				continue
			case <-s.close:
				return
			}
		}
		i = 0

		s.mu.Lock()
		s.stream = st
		s.mu.Unlock()
		err = s.streamReader()
		if err != nil {
			// if stream is error or shutdown, we continue to retry via openStream
			// we exit only in case of s.close, i.e. client component close
			log.Error("streamWatcher: error, continue", zap.Error(err))
		}

	}
}

func (s *subscribeClient) streamReader() error {
	for {
		event, err := s.stream.WaitNotifyEvents()
		if err != nil {
			return err
		}
		// disapatch event to all listeners for this type
		event.
			s.messageReceiver(event)
	}
}
