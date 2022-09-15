package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type SyncService interface {
	NotifyHeadUpdate(ctx context.Context, treeId string, header *aclpb.TreeHeader, update *spacesyncproto.ObjectHeadUpdate) (err error)
	StreamPool() StreamPool
	Close() (err error)
}

type syncService struct {
	syncHandler   SyncHandler
	streamPool    StreamPool
	configuration nodeconf.Configuration
	spaceId       string
}

func (s *syncService) Close() (err error) {
	return s.streamPool.Close()
}

func (s *syncService) NotifyHeadUpdate(ctx context.Context, treeId string, header *aclpb.TreeHeader, update *spacesyncproto.ObjectHeadUpdate) (err error) {
	msg := spacesyncproto.WrapHeadUpdate(update, header, treeId)
	peers, err := s.configuration.AllPeers(context.Background(), s.spaceId)
	if err != nil {
		return
	}
	for _, peer := range peers {
		if s.streamPool.HasStream(peer.Id()) {
			continue
		}
		cl := spacesyncproto.NewDRPCSpaceClient(peer)
		stream, err := cl.Stream(ctx)
		if err != nil {
			continue
		}

		err = s.streamPool.AddAndReadStream(stream)
		if err != nil {
			continue
		}
	}
	return s.streamPool.BroadcastAsync(msg)
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
