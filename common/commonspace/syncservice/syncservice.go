package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectgetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/periodicsync"
	"go.uber.org/zap"
	"time"
)

var log = logger.NewNamed("syncservice").Sugar()

type SyncService interface {
	ocache.ObjectLastUsage
	synchandler.SyncHandler
	StreamPool() StreamPool
	StreamChecker() StreamChecker

	Init(getter objectgetter.ObjectGetter)
	Close() (err error)
}

const respPeersStreamCheckInterval = 3000

type syncService struct {
	spaceId string

	streamPool   StreamPool
	checker      StreamChecker
	periodicSync periodicsync.PeriodicSync
	objectGetter objectgetter.ObjectGetter

	syncCtx    context.Context
	cancelSync context.CancelFunc
}

func NewSyncService(
	spaceId string,
	confConnector nodeconf.ConfConnector,
	periodicSeconds int) (syncService SyncService) {
	streamPool := newStreamPool(func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
		return syncService.HandleMessage(ctx, senderId, message)
	})
	clientFactory := spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceClient)
	syncLog := log.Desugar().With(zap.String("id", spaceId))
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
	syncService = newSyncService(
		spaceId,
		streamPool,
		periodicSync,
		checker,
		syncCtx,
		cancel)
	return
}

func newSyncService(
	spaceId string,
	streamPool StreamPool,
	periodicSync periodicsync.PeriodicSync,
	checker StreamChecker,
	syncCtx context.Context,
	cancel context.CancelFunc,
) *syncService {
	return &syncService{
		periodicSync: periodicSync,
		streamPool:   streamPool,
		spaceId:      spaceId,
		checker:      checker,
		syncCtx:      syncCtx,
		cancelSync:   cancel,
	}
}

func (s *syncService) Init(objectGetter objectgetter.ObjectGetter) {
	s.objectGetter = objectGetter
	s.periodicSync.Run()
}

func (s *syncService) Close() (err error) {
	s.periodicSync.Close()
	s.cancelSync()
	return s.streamPool.Close()
}

func (s *syncService) LastUsage() time.Time {
	return s.streamPool.LastUsage()
}

func (s *syncService) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	log.With(zap.String("peerId", senderId), zap.String("objectId", message.ObjectId)).Debug("handling message")
	obj, err := s.objectGetter.GetObject(ctx, message.ObjectId)
	if err != nil {
		return
	}
	return obj.HandleMessage(ctx, senderId, message)
}

func (s *syncService) StreamPool() StreamPool {
	return s.streamPool
}

func (s *syncService) StreamChecker() StreamChecker {
	return s.checker
}
