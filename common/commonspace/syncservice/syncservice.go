package syncservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type SyncService interface {
	NotifyHeadUpdate(treeId string, header *aclpb.TreeHeader, update *spacesyncproto.ObjectHeadUpdate) (err error)
	StreamPool() StreamPool
}

type syncService struct {
	syncHandler   SyncHandler
	streamPool    StreamPool
	configuration nodeconf.Configuration
}

func (s *syncService) NotifyHeadUpdate(treeId string, header *aclpb.TreeHeader, update *spacesyncproto.ObjectHeadUpdate) (err error) {
	msg := spacesyncproto.WrapHeadUpdate(update, header, treeId)
	all
	return s.streamPool.BroadcastAsync(msg)
}

func (s *syncService) StreamPool() StreamPool {
	return s.streamPool
}

func newSyncService() {

}
