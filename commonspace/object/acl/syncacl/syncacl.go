package syncacl

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
)

type SyncAcl struct {
	list.AclList
	synchandler.SyncHandler
	messagePool objectsync.MessagePool
}

func NewSyncAcl(aclList list.AclList, messagePool objectsync.MessagePool) *SyncAcl {
	return &SyncAcl{
		AclList:     aclList,
		SyncHandler: nil,
		messagePool: messagePool,
	}
}
