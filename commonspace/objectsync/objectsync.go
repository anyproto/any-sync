//go:generate mockgen -destination mock_objectsync/mock_objectsync.go github.com/anyproto/any-sync/commonspace/objectsync SyncClient
package objectsync

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objectsync/syncclient"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/util/multiqueue"
	"github.com/cheggaaa/mb/v3"
	"sync/atomic"
	"time"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

const CName = "common.commonspace.objectsync"

var log = logger.NewNamed(CName)

type ObjectSync interface {
	app.ComponentRunnable
}

type HandleMessage struct {
	Id                uint64
	ReceiveTime       time.Time
	StartHandlingTime time.Time
	Deadline          time.Time
	SenderId          string
	Message           *spacesyncproto.ObjectSyncMessage
	PeerCtx           context.Context
}

func (m HandleMessage) LogFields(fields ...zap.Field) []zap.Field {
	return append(fields,
		metric.SpaceId(m.Message.SpaceId),
		metric.ObjectId(m.Message.ObjectId),
		metric.QueueDur(m.StartHandlingTime.Sub(m.ReceiveTime)),
		metric.TotalDur(time.Since(m.ReceiveTime)),
	)
}

type objectSync struct {
	spaceId string

	syncClient    syncclient.SyncClient
	objectGetter  syncobjectgetter.SyncObjectGetter
	configuration nodeconf.NodeConf
	spaceStorage  spacestorage.SpaceStorage
	metric        metric.Metric

	spaceIsDeleted *atomic.Bool
	handleQueue    multiqueue.MultiQueue[HandleMessage]
}

func (s *objectSync) Init(a *app.App) (err error) {
	s.syncClient = a.MustComponent(syncclient.CName).(syncclient.SyncClient)
	s.objectGetter = a.MustComponent(treemanager.CName).(treemanager.TreeManager).(syncobjectgetter.SyncObjectGetter)
	s.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	sharedData := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	s.metric = a.MustComponent(metric.CName).(metric.Metric)
	s.spaceIsDeleted = sharedData.SpaceIsDeleted
	s.spaceId = sharedData.SpaceId
	s.handleQueue = multiqueue.New[HandleMessage](s.processHandleMessage, 100)
	return nil
}

func (s *objectSync) Name() (name string) {
	return CName
}

func (s *objectSync) Run(ctx context.Context) (err error) {
	return nil
}

func (s *objectSync) Close(ctx context.Context) (err error) {
	return s.handleQueue.Close()
}

func NewObjectSync() ObjectSync {
	return &objectSync{}
}

func (s *objectSync) HandleMessage(ctx context.Context, hm HandleMessage) (err error) {
	threadId := hm.Message.ObjectId
	hm.ReceiveTime = time.Now()
	if hm.Message.ReplyId != "" {
		threadId += hm.Message.ReplyId
		defer func() {
			_ = s.handleQueue.CloseThread(threadId)
		}()
	}
	if hm.PeerCtx == nil {
		hm.PeerCtx = ctx
	}
	err = s.handleQueue.Add(ctx, threadId, hm)
	if err == mb.ErrOverflowed {
		log.InfoCtx(ctx, "queue overflowed", zap.String("spaceId", s.spaceId), zap.String("objectId", threadId))
		// skip overflowed error
		return nil
	}
	return
}

func (s *objectSync) processHandleMessage(msg HandleMessage) {
	var err error
	msg.StartHandlingTime = time.Now()
	ctx := peer.CtxWithPeerId(context.Background(), msg.SenderId)
	ctx = logger.CtxWithFields(ctx, zap.Uint64("msgId", msg.Id), zap.String("senderId", msg.SenderId))
	defer func() {
		if s.metric == nil {
			return
		}
		s.metric.RequestLog(msg.PeerCtx, "space.streamOp", msg.LogFields(
			zap.Error(err),
		)...)
	}()

	if !msg.Deadline.IsZero() {
		now := time.Now()
		if now.After(msg.Deadline) {
			log.InfoCtx(ctx, "skip message: deadline exceed")
			err = context.DeadlineExceeded
			return
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, msg.Deadline)
		defer cancel()
	}
	if err = s.handleMessage(ctx, msg.SenderId, msg.Message); err != nil {
		if msg.Message.ObjectId != "" {
			// cleanup thread on error
			_ = s.handleQueue.CloseThread(msg.Message.ObjectId)
		}
		log.InfoCtx(ctx, "handleMessage error", zap.Error(err))
	}
}

func (s *objectSync) handleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	log := log.With(
		zap.String("objectId", msg.ObjectId),
		zap.String("requestId", msg.RequestId),
		zap.String("replyId", msg.ReplyId))
	if s.spaceIsDeleted.Load() {
		log = log.With(zap.Bool("isDeleted", true))
		// preventing sync with other clients if they are not just syncing the settings tree
		if !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) && msg.ObjectId != s.spaceStorage.SpaceSettingsId() {
			return fmt.Errorf("can't perform operation with object, space is deleted")
		}
	}
	obj, err := s.objectGetter.GetObject(ctx, msg.ObjectId)
	if err != nil {
		return fmt.Errorf("failed to get object from cache: %w", err)
	}
	err = obj.HandleMessage(ctx, senderId, msg)
	if err != nil {
		return fmt.Errorf("failed to handle message: %w", err)
	}
	return
}
