package sync

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/util/multiqueue"
)

const CName = "common.commonspace.sync"

var log = logger.NewNamed("sync")

type SyncService interface {
	GetQueueProvider() multiqueue.QueueProvider[drpc.Message]
}

type MergeFilterFunc func(ctx context.Context, msg drpc.Message, q *mb.MB[drpc.Message]) error

type syncService struct {
	sendQueueProvider multiqueue.QueueProvider[drpc.Message]
	receiveQueue      multiqueue.MultiQueue[drpc.Message]
	manager           RequestManager
	handler           HeadUpdateHandler
	sender            HeadUpdateSender
	mergeFilter       MergeFilterFunc
	ctx               context.Context
	cancel            context.CancelFunc
}

func NewSyncService(deps SyncDeps) SyncService {
	s := &syncService{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.sendQueueProvider = multiqueue.NewQueueProvider[drpc.Message](100, s.handleOutgoingMessage)
	s.receiveQueue = multiqueue.New[drpc.Message](s.handleIncomingMessage, 100)
	s.sender = deps.HeadUpdateSender
	s.handler = deps.HeadUpdateHandler
	s.mergeFilter = deps.MergeFilter
	s.manager = NewRequestManager(deps)
	return s
}

func (s *syncService) handleOutgoingMessage(id string, msg drpc.Message, q *mb.MB[drpc.Message]) error {
	return s.mergeFilter(s.ctx, msg, q)
}

func (s *syncService) handleIncomingMessage(msg drpc.Message) {
	req, err := s.handler.HandleHeadUpdate(s.ctx, msg)
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

func (s *syncService) GetQueueProvider() multiqueue.QueueProvider[drpc.Message] {
	return s.sendQueueProvider
}

func (s *syncService) HandleMessage(ctx context.Context, peerId string, msg drpc.Message) error {
	return s.receiveQueue.Add(ctx, peerId, msg)
}

func (s *syncService) HandleStreamRequest(ctx context.Context, req Request, stream drpc.Stream) error {
	return s.manager.HandleStreamRequest(req, stream)
}

func (s *syncService) NewReadMessage() drpc.Message {
	return &HeadUpdate{}
}
