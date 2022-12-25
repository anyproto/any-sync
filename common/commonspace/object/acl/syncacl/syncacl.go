package syncacl

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync/synchandler"
)

type SyncACL struct {
	list.ACLList
	synchandler.SyncHandler
	streamPool objectsync.StreamPool
}

func NewSyncACL(aclList list.ACLList, streamPool objectsync.StreamPool) *SyncACL {
	return &SyncACL{
		ACLList:     aclList,
		SyncHandler: nil,
		streamPool:  streamPool,
	}
}
