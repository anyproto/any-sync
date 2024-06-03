package sync

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/util/multiqueue"
)

const CName = "common.commonspace.sync"

var log = logger.NewNamed("sync")

type SyncService interface {
	app.Component
	GetQueue(peerId string) *multiqueue.Queue[drpc.Message]
	HandleMessage(ctx context.Context, peerId string, msg drpc.Message) error
	HandleStreamRequest(ctx context.Context, req syncdeps.Request, stream drpc.Stream) error
	QueueRequest(ctx context.Context, rq syncdeps.Request) error
}

type syncService struct {
	sendQueueProvider multiqueue.QueueProvider[drpc.Message]
	receiveQueue      multiqueue.MultiQueue[msgCtx]
	manager           RequestManager
	handler           syncdeps.HeadUpdateHandler
	mergeFilter       syncdeps.MergeFilterFunc
	newMessage        func() drpc.Message
	ctx               context.Context
	cancel            context.CancelFunc
}

type msgCtx struct {
	ctx context.Context
	drpc.Message
}

func (s *syncService) Init(a *app.App) (err error) {
	factory := a.MustComponent(syncdeps.CName).(syncdeps.SyncDepsFactory)
	s.sendQueueProvider = multiqueue.NewQueueProvider[drpc.Message](100, s.handleOutgoingMessage)
	s.receiveQueue = multiqueue.New[msgCtx](s.handleIncomingMessage, 100)
	deps := factory.SyncDeps()
	s.handler = deps.HeadUpdateHandler
	s.mergeFilter = deps.MergeFilter
	s.newMessage = deps.ReadMessageConstructor
	s.manager = NewRequestManager(deps)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return nil
}

func (s *syncService) Name() (name string) {
	return CName
}

func NewSyncService() SyncService {
	return &syncService{}
}

func (s *syncService) handleOutgoingMessage(id string, msg drpc.Message, q *mb.MB[drpc.Message]) error {
	return s.mergeFilter(s.ctx, msg, q)
}

func (s *syncService) handleIncomingMessage(msg msgCtx) {
	req, err := s.handler.HandleHeadUpdate(msg.ctx, msg.Message)
	if err != nil {
		log.Error("failed to handle head update", zap.Error(err))
	}
	if req == nil {
		return
	}
	err = s.manager.QueueRequest(req)
	if err != nil {
		log.Error("failed to queue request", zap.Error(err))
	}
}

func (s *syncService) GetQueue(peerId string) *multiqueue.Queue[drpc.Message] {
	queue := s.sendQueueProvider.GetQueue(peerId)
	return queue
}

func (s *syncService) NewReadMessage() drpc.Message {
	return s.newMessage()
}

func (s *syncService) HandleMessage(ctx context.Context, peerId string, msg drpc.Message) error {
	return s.receiveQueue.Add(ctx, peerId, msgCtx{
		ctx:     ctx,
		Message: msg,
	})
}

func (s *syncService) QueueRequest(ctx context.Context, rq syncdeps.Request) error {
	return s.manager.QueueRequest(rq)
}

func (s *syncService) HandleStreamRequest(ctx context.Context, req syncdeps.Request, stream drpc.Stream) error {
	return s.manager.HandleStreamRequest(ctx, req, stream)
}
