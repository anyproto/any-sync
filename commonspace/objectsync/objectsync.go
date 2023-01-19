//go:generate mockgen -destination mock_objectsync/mock_objectsync.go github.com/anytypeio/any-sync/commonspace/objectsync ActionQueue
package objectsync

import (
	"context"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/app/ocache"
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
	MessagePool() MessagePool
	ActionQueue() ActionQueue

	Init(getter syncobjectgetter.SyncObjectGetter)
	Close() (err error)
}

type objectSync struct {
	spaceId string

	streamPool   MessagePool
	objectGetter syncobjectgetter.SyncObjectGetter
	actionQueue  ActionQueue

	syncCtx    context.Context
	cancelSync context.CancelFunc
}

func NewObjectSync(streamManager StreamManager, spaceId string) (objectSync ObjectSync) {
	msgPool := newMessagePool(streamManager, func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
		return objectSync.HandleMessage(ctx, senderId, message)
	})
	syncCtx, cancel := context.WithCancel(context.Background())
	objectSync = newObjectSync(
		spaceId,
		msgPool,
		syncCtx,
		cancel)
	return
}

func newObjectSync(
	spaceId string,
	streamPool MessagePool,
	syncCtx context.Context,
	cancel context.CancelFunc,
) *objectSync {
	return &objectSync{
		streamPool:  streamPool,
		spaceId:     spaceId,
		syncCtx:     syncCtx,
		cancelSync:  cancel,
		actionQueue: NewDefaultActionQueue(),
	}
}

func (s *objectSync) Init(objectGetter syncobjectgetter.SyncObjectGetter) {
	s.objectGetter = objectGetter
	s.actionQueue.Run()
}

func (s *objectSync) Close() (err error) {
	s.actionQueue.Close()
	s.cancelSync()
	return
}

func (s *objectSync) LastUsage() time.Time {
	// TODO: [che]
	return time.Now()
}

func (s *objectSync) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	log.With(zap.String("peerId", senderId), zap.String("objectId", message.ObjectId)).Debug("handling message")
	obj, err := s.objectGetter.GetObject(ctx, message.ObjectId)
	if err != nil {
		return
	}
	return obj.HandleMessage(ctx, senderId, message)
}

func (s *objectSync) MessagePool() MessagePool {
	return s.streamPool
}

func (s *objectSync) ActionQueue() ActionQueue {
	return s.actionQueue
}
