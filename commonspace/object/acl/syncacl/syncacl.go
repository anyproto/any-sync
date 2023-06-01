package syncacl

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
)

type SyncAcl struct {
	list.AclList
	synchandler.SyncHandler
}

func NewSyncAcl(aclList list.AclList) *SyncAcl {
	return &SyncAcl{
		AclList:     aclList,
		SyncHandler: nil,
	}
}
