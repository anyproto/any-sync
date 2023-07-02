package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
)

type syncAclHandler struct {
	aclList      list.AclList
	syncClient   SyncClient
	syncProtocol AclSyncProtocol
	syncStatus   syncstatus.StatusUpdater
	spaceId      string
}

func newSyncAclHandler(spaceId string, aclList list.AclList, syncClient SyncClient, syncStatus syncstatus.StatusUpdater) synchandler.SyncHandler {
	return &syncAclHandler{
		aclList:      aclList,
		syncClient:   syncClient,
		syncProtocol: newAclSyncProtocol(spaceId, aclList, syncClient),
		syncStatus:   syncStatus,
		spaceId:      spaceId,
	}
}

func (s *syncAclHandler) HandleMessage(ctx context.Context, senderId string, req *spacesyncproto.ObjectSyncMessage) (err error) {
	return
}

func (s *syncAclHandler) HandleRequest(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (response *spacesyncproto.ObjectSyncMessage, err error) {
	return
}
