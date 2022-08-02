package message

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/requesthandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/syncpb"
	"github.com/cheggaaa/mb"
	"go.uber.org/zap"
	"sync"
)

var log = logger.NewNamed("messageservice")

const CName = "MessageService"

type service struct {
	receiveBatcher *mb.MB
	sendBatcher    *mb.MB
	senderChannels map[string]chan *syncpb.SyncContent
	requestHandler requesthandler.RequestHandler
	sync.RWMutex
}

type message struct {
	peerId  string
	content *syncpb.SyncContent
}

func New() app.Component {
	return &service{}
}

type Service interface {
	RegisterMessageSender(peerId string) chan *syncpb.SyncContent
	UnregisterMessageSender(peerId string)

	HandleMessage(peerId string, msg *syncpb.SyncContent) error
	SendMessage(peerId string, msg *syncpb.SyncContent) error
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.receiveBatcher = mb.New(0)
	s.sendBatcher = mb.New(0)
	s.senderChannels = make(map[string]chan *syncpb.SyncContent)
	s.requestHandler = a.MustComponent(requesthandler.CName).(requesthandler.RequestHandler)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	go s.runSender(ctx)
	go s.runReceiver(ctx)
	return nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) RegisterMessageSender(peerId string) chan *syncpb.SyncContent {
	s.Lock()
	defer s.Unlock()
	if ch, exists := s.senderChannels[peerId]; !exists {
		return ch
	}
	ch := make(chan *syncpb.SyncContent)
	s.senderChannels[peerId] = ch
	return ch
}

func (s *service) UnregisterMessageSender(peerId string) {
	s.Lock()
	defer s.Unlock()
	if _, exists := s.senderChannels[peerId]; !exists {
		return
	}
	close(s.senderChannels[peerId])
	delete(s.senderChannels, peerId)
}

func (s *service) HandleMessage(peerId string, msg *syncpb.SyncContent) error {
	return s.receiveBatcher.Add(&message{
		peerId:  peerId,
		content: msg,
	})
}

func (s *service) SendMessage(peerId string, msg *syncpb.SyncContent) error {
	return s.sendBatcher.Add(&message{
		peerId:  peerId,
		content: msg,
	})
}

func (s *service) runReceiver(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}
		msgs := s.receiveBatcher.WaitMinMax(1, 100)
		// TODO: this is bad to serve everything on a new goroutine, but very easy for prototyping :-)
		for _, msg := range msgs {
			typedMsg := msg.(*message)
			go func(typedMsg *message) {
				err := s.requestHandler.HandleFullSyncContent(ctx, typedMsg.peerId, typedMsg.content)
				if err != nil {
					log.Error("failed to handle content", zap.Error(err))
				}
			}(typedMsg)
		}
	}
}

func (s *service) runSender(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}
		msgs := s.sendBatcher.WaitMinMax(1, 100)
		s.RLock()
		for _, msg := range msgs {
			s.sendMessage(msg.(*message))
		}
		s.RUnlock()
	}
}

func (s *service) sendMessage(typedMsg *message) {
	// this should be done under lock
	if typedMsg.content.GetMessage().GetHeadUpdate() != nil {
		for _, ch := range s.senderChannels {
			ch <- typedMsg.content
		}
		return
	}
	ch, exists := s.senderChannels[typedMsg.peerId]
	if !exists {
		return
	}
	ch <- typedMsg.content
}
