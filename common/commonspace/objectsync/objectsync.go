//go:generate mockgen -destination mock_objectsync/mock_objectsync.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync ActionQueue
package objectsync

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/confconnector"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/periodicsync"
	"go.uber.org/zap"
	"time"
)

var log = logger.NewNamed("commonspace.objectsync")

type ObjectSync interface {
	ocache.ObjectLastUsage
	synchandler.SyncHandler
	StreamPool() StreamPool
	StreamChecker() StreamChecker
	ActionQueue() ActionQueue

	Init(getter syncobjectgetter.SyncObjectGetter)
	Close() (err error)
}

type objectSync struct {
	spaceId string

	streamPool   StreamPool
	checker      StreamChecker
	periodicSync periodicsync.PeriodicSync
	objectGetter syncobjectgetter.SyncObjectGetter
	actionQueue  ActionQueue

	syncCtx    context.Context
	cancelSync context.CancelFunc
}

func NewObjectSync(
	spaceId string,
	confConnector confconnector.ConfConnector,
	periodicSeconds int) (objectSync ObjectSync) {
	streamPool := newStreamPool(func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
		return objectSync.HandleMessage(ctx, senderId, message)
	})
	clientFactory := spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceSyncClient)
	syncLog := log.With(zap.String("id", spaceId))
	syncCtx, cancel := context.WithCancel(context.Background())
	checker := NewStreamChecker(
		spaceId,
		confConnector,
		streamPool,
		clientFactory,
		syncCtx,
		syncLog)
	periodicSync := periodicsync.NewPeriodicSync(periodicSeconds, 0, func(ctx context.Context) error {
		checker.CheckResponsiblePeers()
		return nil
	}, syncLog)
	objectSync = newObjectSync(
		spaceId,
		streamPool,
		periodicSync,
		checker,
		syncCtx,
		cancel)
	return
}

func newObjectSync(
	spaceId string,
	streamPool StreamPool,
	periodicSync periodicsync.PeriodicSync,
	checker StreamChecker,
	syncCtx context.Context,
	cancel context.CancelFunc,
) *objectSync {
	return &objectSync{
		periodicSync: periodicSync,
		streamPool:   streamPool,
		spaceId:      spaceId,
		checker:      checker,
		syncCtx:      syncCtx,
		cancelSync:   cancel,
		actionQueue:  NewActionQueue(maxStreamReaders, 100),
	}
}

func (s *objectSync) Init(objectGetter syncobjectgetter.SyncObjectGetter) {
	s.objectGetter = objectGetter
	s.actionQueue.Run()
	s.periodicSync.Run()
}

func (s *objectSync) Close() (err error) {
	s.actionQueue.Close()
	s.periodicSync.Close()
	s.cancelSync()
	return s.streamPool.Close()
}

func (s *objectSync) LastUsage() time.Time {
	return s.streamPool.LastUsage()
}

func (s *objectSync) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	log.With(zap.String("peerId", senderId), zap.String("objectId", message.ObjectId)).Debug("handling message")
	obj, err := s.objectGetter.GetObject(ctx, message.ObjectId)
	if err != nil {
		return
	}
	return obj.HandleMessage(ctx, senderId, message)
}

func (s *objectSync) StreamPool() StreamPool {
	return s.streamPool
}

func (s *objectSync) StreamChecker() StreamChecker {
	return s.checker
}

func (s *objectSync) ActionQueue() ActionQueue {
	return s.actionQueue
}
