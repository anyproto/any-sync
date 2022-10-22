package syncacl

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
)

type SyncACL struct {
	list.ACLList
	synchandler.SyncHandler
	streamPool syncservice.StreamPool
}

func NewSyncACL(aclList list.ACLList, streamPool syncservice.StreamPool) *SyncACL {
	return &SyncACL{
		ACLList:     aclList,
		SyncHandler: nil,
		streamPool:  streamPool,
	}
}
