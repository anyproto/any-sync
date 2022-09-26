package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
	"time"
)

type SyncService interface {
	NotifyHeadUpdate(
		ctx context.Context,
		treeId string,
		root *treechangeproto.RawTreeChangeWithId,
		update *spacesyncproto.ObjectHeadUpdate) (err error)
	StreamPool() StreamPool

	Init()
	Close() (err error)
}

type HeadNotifiable interface {
	UpdateHeads(id string, heads []string)
}

const respPeersStreamCheckInterval = time.Second * 10

type syncService struct {
	spaceId string

	syncHandler    SyncHandler
	streamPool     StreamPool
	headNotifiable HeadNotifiable
	configuration  nodeconf.Configuration

	streamLoopCtx  context.Context
	stopStreamLoop context.CancelFunc
	streamLoopDone chan struct{}
}

func NewSyncService(spaceId string, headNotifiable HeadNotifiable, cache cache.TreeCache, configuration nodeconf.Configuration) SyncService {
	var syncHandler SyncHandler
	streamPool := newStreamPool(func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
		return syncHandler.HandleMessage(ctx, senderId, message)
	})
	syncHandler = newSyncHandler(cache, streamPool)
	return newSyncService(spaceId, headNotifiable, syncHandler, streamPool, configuration)
}

func newSyncService(
	spaceId string,
	headNotifiable HeadNotifiable,
	syncHandler SyncHandler,
	streamPool StreamPool,
	configuration nodeconf.Configuration) *syncService {
	return &syncService{
		syncHandler:    syncHandler,
		streamPool:     streamPool,
		headNotifiable: headNotifiable,
		configuration:  configuration,
		spaceId:        spaceId,
		streamLoopDone: make(chan struct{}),
	}
}

func (s *syncService) Init() {
	s.streamLoopCtx, s.stopStreamLoop = context.WithCancel(context.Background())
	go s.responsibleStreamCheckLoop(s.streamLoopCtx)
}

func (s *syncService) Close() (err error) {
	s.stopStreamLoop()
	<-s.streamLoopDone
	return s.streamPool.Close()
}

func (s *syncService) NotifyHeadUpdate(
	ctx context.Context,
	treeId string,
	header *treechangeproto.RawTreeChangeWithId,
	update *spacesyncproto.ObjectHeadUpdate) (err error) {
	s.headNotifiable.UpdateHeads(treeId, update.Heads)
	return s.streamPool.BroadcastAsync(spacesyncproto.WrapHeadUpdate(update, header, treeId, ""))
}

func (s *syncService) responsibleStreamCheckLoop(ctx context.Context) {
	defer close(s.streamLoopDone)
	checkResponsiblePeers := func() {
		respPeers, err := s.configuration.ResponsiblePeers(ctx, s.spaceId)
		if err != nil {
			return
		}
		for _, peer := range respPeers {
			if s.streamPool.HasActiveStream(peer.Id()) {
				continue
			}
			cl := spacesyncproto.NewDRPCSpaceClient(peer)
			stream, err := cl.Stream(ctx)
			if err != nil {
				continue
			}

			s.streamPool.AddAndReadStreamAsync(stream)
		}
	}

	checkResponsiblePeers()
	ticker := time.NewTicker(respPeersStreamCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.streamLoopCtx.Done():
			return
		case <-ticker.C:
			checkResponsiblePeers()
		}
	}
}

func (s *syncService) StreamPool() StreamPool {
	return s.streamPool
}
