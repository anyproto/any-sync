package sync

import (
	"context"
	"errors"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/multiqueue"
	"github.com/anyproto/any-sync/util/syncqueues"
)

const CName = "common.commonspace.sync"

var log = logger.NewNamed("sync")

var ErrUnexpectedMessage = errors.New("unexpected message")

type SyncService interface {
	app.Component
	BroadcastMessage(ctx context.Context, msg drpc.Message) error
	HandleDeprecatedObjectSync(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error)
	HandleStreamRequest(ctx context.Context, req syncdeps.Request, stream drpc.Stream) error
	HandleMessage(ctx context.Context, msg drpc.Message) error
	SendRequest(ctx context.Context, rq syncdeps.Request, collector syncdeps.ResponseCollector) error
	QueueRequest(ctx context.Context, rq syncdeps.Request) error
	CloseReceiveQueue(id string) error
}

type syncService struct {
	receiveQueue multiqueue.MultiQueue[msgCtx]
	manager      RequestManager
	streamPool   streampool.StreamPool
	peerManager  peermanager.PeerManager
	nodeConf     nodeconf.NodeConf
	handler      syncdeps.SyncHandler
	spaceId      string
	metric       *syncMetric
	commonMetric metric.Metric
	ctx          context.Context
	cancel       context.CancelFunc
}

type msgCtx struct {
	ctx context.Context
	multiqueue.Sizeable
}

func (m msgCtx) MsgSize() uint64 {
	return m.Sizeable.MsgSize()
}

func NewSyncService() SyncService {
	return &syncService{}
}

func (s *syncService) Name() (name string) {
	return CName
}

func (s *syncService) Init(a *app.App) (err error) {
	s.metric = &syncMetric{}
	s.spaceId = a.MustComponent(spacestate.CName).(*spacestate.SpaceState).SpaceId
	s.handler = a.MustComponent(syncdeps.CName).(syncdeps.SyncHandler)
	s.receiveQueue = multiqueue.New[msgCtx](s.handleIncomingMessage, s.metric, syncdeps.MsgTypeIncoming, 100)
	s.peerManager = a.MustComponent(peermanager.CName).(peermanager.PeerManager)
	s.streamPool = a.MustComponent(streampool.CName).(streampool.StreamPool)
	s.commonMetric, _ = a.Component(metric.CName).(metric.Metric)
	syncQueues := a.MustComponent(syncqueues.CName).(syncqueues.SyncQueues)
	s.manager = NewRequestManager(s.handler, s.metric, syncQueues.ActionPool(s.spaceId), syncQueues.Limit(s.spaceId))
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return nil
}

func (s *syncService) Run(ctx context.Context) (err error) {
	if s.commonMetric != nil {
		s.commonMetric.RegisterSyncMetric(s.spaceId, s.metric)
	}
	return nil
}

func (s *syncService) Close(ctx context.Context) (err error) {
	err = s.receiveQueue.Close()
	if s.commonMetric != nil {
		s.commonMetric.UnregisterSyncMetric(s.spaceId, s.metric)
	}
	s.manager.Close()
	return
}

func (s *syncService) BroadcastMessage(ctx context.Context, msg drpc.Message) error {
	return s.peerManager.BroadcastMessage(ctx, msg)
}

func (s *syncService) handleIncomingMessage(msg msgCtx) {
	req, err := s.handler.HandleHeadUpdate(msg.ctx, msg.Sizeable)
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
	//msg.StartHandlingTime = time.Now()
	//ctx := peer.CtxWithPeerId(context.Background(), msg.SenderId)
	//ctx = logger.CtxWithFields(ctx, zap.Uint64("msgId", msg.Id), zap.String("senderId", msg.SenderId))
	//s.metric.RequestLog(msg.PeerCtx, "space.streamOp", msg.LogFields(
	//	zap.Error(err),
	//)...)
}

func (s *syncService) HandleMessage(ctx context.Context, msg drpc.Message) error {
	idMsg, ok := msg.(syncdeps.Message)
	if !ok {
		return ErrUnexpectedMessage
	}
	objectId := idMsg.ObjectId()
	err := s.receiveQueue.Add(ctx, objectId, msgCtx{
		ctx:      ctx,
		Sizeable: idMsg,
	})
	if errors.Is(err, mb.ErrOverflowed) {
		log.Info("queue overflowed", zap.String("objectId", objectId))
		return nil
	}
	return err
}

func (s *syncService) HandleDeprecatedObjectSync(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error) {
	return s.manager.HandleDeprecatedObjectSync(ctx, req)
}

func (s *syncService) QueueRequest(ctx context.Context, rq syncdeps.Request) error {
	return s.manager.QueueRequest(rq)
}

func (s *syncService) SendRequest(ctx context.Context, rq syncdeps.Request, collector syncdeps.ResponseCollector) error {
	return s.manager.SendRequest(ctx, rq, collector)
}

func (s *syncService) HandleStreamRequest(ctx context.Context, req syncdeps.Request, stream drpc.Stream) error {
	return s.manager.HandleStreamRequest(ctx, req, stream)
}

func (s *syncService) CloseReceiveQueue(id string) error {
	return s.receiveQueue.CloseThread(id)
}
