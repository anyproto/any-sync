package syncacl

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
)

type SyncACL struct {
	list.ACLList
	syncservice.SyncClient
	synchandler.SyncHandler
}

func NewSyncACL(aclList list.ACLList, syncClient syncservice.SyncClient) *SyncACL {
	return &SyncACL{
		ACLList:     aclList,
		SyncClient:  syncClient,
		SyncHandler: nil,
	}
}
