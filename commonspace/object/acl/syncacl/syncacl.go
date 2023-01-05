package syncacl

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
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
