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

	Init()
	Close() (err error)
}

type objectSync struct {
	spaceId string

	messagePool  MessagePool
	objectGetter syncobjectgetter.SyncObjectGetter

	syncCtx    context.Context
	cancelSync context.CancelFunc
}

func NewObjectSync(
	spaceId string,
	streamManager StreamManager,
	objectGetter syncobjectgetter.SyncObjectGetter) ObjectSync {
	syncCtx, cancel := context.WithCancel(context.Background())
	os := newObjectSync(
		spaceId,
		objectGetter,
		syncCtx,
		cancel)
	msgPool := newMessagePool(streamManager, os.handleMessage)
	os.messagePool = msgPool
	return os
}

func newObjectSync(
	spaceId string,
	objectGetter syncobjectgetter.SyncObjectGetter,
	syncCtx context.Context,
	cancel context.CancelFunc,
) *objectSync {
	return &objectSync{
		objectGetter: objectGetter,
		spaceId:      spaceId,
		syncCtx:      syncCtx,
		cancelSync:   cancel,
		//actionQueue:  NewDefaultActionQueue(),
	}
}

func (s *objectSync) Init() {
	//s.actionQueue.Run()
}

func (s *objectSync) Close() (err error) {
	//s.actionQueue.Close()
	s.cancelSync()
	return
}

func (s *objectSync) LastUsage() time.Time {
	return s.messagePool.LastUsage()
}

func (s *objectSync) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	return s.messagePool.HandleMessage(ctx, senderId, message)
}

func (s *objectSync) handleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	log.With(zap.String("peerId", senderId), zap.String("objectId", message.ObjectId), zap.String("replyId", message.ReplyId)).Debug("handling message")
	obj, err := s.objectGetter.GetObject(ctx, message.ObjectId)
	if err != nil {
		return
	}
	return obj.HandleMessage(ctx, senderId, message)
}

func (s *objectSync) MessagePool() MessagePool {
	return s.messagePool
}
