package syncacl

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync/synchandler"
)

type SyncAcl struct {
	list.AclList
	synchandler.SyncHandler
	streamPool objectsync.StreamPool
}

func NewSyncAcl(aclList list.AclList, streamPool objectsync.StreamPool) *SyncAcl {
	return &SyncAcl{
		AclList:     aclList,
		SyncHandler: nil,
		streamPool:  streamPool,
	}
}
