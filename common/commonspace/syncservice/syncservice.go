package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"time"
)

type SyncService interface {
	NotifyHeadUpdate(ctx context.Context, treeId string, header *aclpb.TreeHeader, update *spacesyncproto.ObjectHeadUpdate) (err error)
	StreamPool() StreamPool
	Close() (err error)
}

const respPeersStreamCheckInterval = time.Second * 10

type syncService struct {
	syncHandler    SyncHandler
	streamPool     StreamPool
	configuration  nodeconf.Configuration
	spaceId        string
	streamLoopCtx  context.Context
	stopStreamLoop context.CancelFunc
}

func (s *syncService) Run() {
	s.streamLoopCtx, s.stopStreamLoop = context.WithCancel(context.Background())
	s.streamCheckLoop(s.streamLoopCtx)
}

func (s *syncService) Close() (err error) {
	s.stopStreamLoop()
	return s.streamPool.Close()
}

func (s *syncService) NotifyHeadUpdate(ctx context.Context, treeId string, header *aclpb.TreeHeader, update *spacesyncproto.ObjectHeadUpdate) (err error) {
	return s.streamPool.BroadcastAsync(spacesyncproto.WrapHeadUpdate(update, header, treeId))
}

func (s *syncService) streamCheckLoop(ctx context.Context) {
	for {
		respPeers, err := s.configuration.ResponsiblePeers(ctx, s.spaceId)
		if err != nil {
			continue
		}
		for _, peer := range respPeers {
			if s.streamPool.HasStream(peer.Id()) {
				continue
			}
			cl := spacesyncproto.NewDRPCSpaceClient(peer)
			stream, err := cl.Stream(ctx)
			if err != nil {
				continue
			}

			s.streamPool.AddAndReadStreamAsync(stream)
		}
		select {
		case <-time.After(respPeersStreamCheckInterval):
			break
		case <-ctx.Done():
			return
		}
	}
}

func (s *syncService) StreamPool() StreamPool {
	return s.streamPool
}

func NewSyncService(spaceId string, cache cache.TreeCache, configuration nodeconf.Configuration) SyncService {
	var syncHandler SyncHandler
	streamPool := newStreamPool(func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
		return syncHandler.HandleMessage(ctx, senderId, message)
	})
	syncHandler = newSyncHandler(cache, streamPool)
	return newSyncService(spaceId, syncHandler, streamPool, configuration)
}

func newSyncService(
	spaceId string,
	syncHandler SyncHandler,
	streamPool StreamPool,
	configuration nodeconf.Configuration) *syncService {
	return &syncService{
		syncHandler:   syncHandler,
		streamPool:    streamPool,
		configuration: configuration,
		spaceId:       spaceId,
	}
}
