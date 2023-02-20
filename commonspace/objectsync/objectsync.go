package objectsync

import (
	"context"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/app/ocache"
	"github.com/anytypeio/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"go.uber.org/zap"
	"sync/atomic"
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

	syncCtx        context.Context
	cancelSync     context.CancelFunc
	spaceIsDeleted *atomic.Bool
}

func NewObjectSync(
	spaceId string,
	spaceIsDeleted *atomic.Bool,
	peerManager peermanager.PeerManager,
	objectGetter syncobjectgetter.SyncObjectGetter) ObjectSync {
	syncCtx, cancel := context.WithCancel(context.Background())
	os := newObjectSync(
		spaceId,
		spaceIsDeleted,
		objectGetter,
		syncCtx,
		cancel)
	msgPool := newMessagePool(peerManager, os.handleMessage)
	os.messagePool = msgPool
	return os
}

func newObjectSync(
	spaceId string,
	spaceIsDeleted *atomic.Bool,
	objectGetter syncobjectgetter.SyncObjectGetter,
	syncCtx context.Context,
	cancel context.CancelFunc,
) *objectSync {
	return &objectSync{
		objectGetter:   objectGetter,
		spaceId:        spaceId,
		syncCtx:        syncCtx,
		cancelSync:     cancel,
		spaceIsDeleted: spaceIsDeleted,
	}
}

func (s *objectSync) Init() {
}

func (s *objectSync) Close() (err error) {
	s.cancelSync()
	return
}

func (s *objectSync) LastUsage() time.Time {
	return s.messagePool.LastUsage()
}

func (s *objectSync) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	return s.messagePool.HandleMessage(ctx, senderId, message)
}

func (s *objectSync) handleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	if s.spaceIsDeleted.Load() {
		return spacesyncproto.ErrSpaceIsDeleted
	}
	log.With(zap.String("objectId", msg.ObjectId), zap.String("replyId", msg.ReplyId)).DebugCtx(ctx, "handling message")
	obj, err := s.objectGetter.GetObject(ctx, msg.ObjectId)
	if err != nil {
		return
	}
	return obj.HandleMessage(ctx, senderId, msg)
}

func (s *objectSync) MessagePool() MessagePool {
	return s.messagePool
}
