//go:generate mockgen -destination mock_objectsync/mock_objectsync.go github.com/anytypeio/any-sync/commonspace/objectsync ActionQueue
package objectsync

import (
	"context"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/app/ocache"
	"github.com/anytypeio/any-sync/commonspace/confconnector"
	"github.com/anytypeio/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
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
	objectGetter syncobjectgetter.SyncObjectGetter
	actionQueue  ActionQueue

	syncCtx    context.Context
	cancelSync context.CancelFunc
}

func NewObjectSync(
	spaceId string,
	confConnector confconnector.ConfConnector) (objectSync ObjectSync) {
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
	objectSync = newObjectSync(
		spaceId,
		streamPool,
		checker,
		syncCtx,
		cancel)
	return
}

func newObjectSync(
	spaceId string,
	streamPool StreamPool,
	checker StreamChecker,
	syncCtx context.Context,
	cancel context.CancelFunc,
) *objectSync {
	return &objectSync{
		streamPool:  streamPool,
		spaceId:     spaceId,
		checker:     checker,
		syncCtx:     syncCtx,
		cancelSync:  cancel,
		actionQueue: NewDefaultActionQueue(),
	}
}

func (s *objectSync) Init(objectGetter syncobjectgetter.SyncObjectGetter) {
	s.objectGetter = objectGetter
	s.actionQueue.Run()
	go s.checker.CheckResponsiblePeers()
}

func (s *objectSync) Close() (err error) {
	s.actionQueue.Close()
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
