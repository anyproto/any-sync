package objectsync

import (
	"context"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/app/ocache"
	"github.com/anytypeio/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/nodeconf"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
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

	messagePool   MessagePool
	objectGetter  syncobjectgetter.SyncObjectGetter
	configuration nodeconf.Configuration

	syncCtx        context.Context
	cancelSync     context.CancelFunc
	spaceIsDeleted *atomic.Bool
}

func NewObjectSync(
	spaceId string,
	spaceIsDeleted *atomic.Bool,
	configuration nodeconf.Configuration,
	peerManager peermanager.PeerManager,
	objectGetter syncobjectgetter.SyncObjectGetter) ObjectSync {
	syncCtx, cancel := context.WithCancel(context.Background())
	os := newObjectSync(
		spaceId,
		spaceIsDeleted,
		configuration,
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
	configuration nodeconf.Configuration,
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
		configuration:  configuration,
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
	log := log.With(zap.String("objectId", msg.ObjectId), zap.String("replyId", msg.ReplyId))
	if s.spaceIsDeleted.Load() {
		log = log.With(zap.Bool("isDeleted", true))
		// preventing sync with other clients
		if !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) {
			return spacesyncproto.ErrSpaceIsDeleted
		}
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
