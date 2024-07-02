package syncacl

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

var (
	ErrMessageIsRequest    = errors.New("message is request")
	ErrMessageIsNotRequest = errors.New("message is not request")
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

func (s *syncAclHandler) HandleMessage(ctx context.Context, senderId string, protoVersion uint32, message *spacesyncproto.ObjectSyncMessage) (err error) {
	unmarshalled := &consensusproto.LogSyncMessage{}
	err = proto.Unmarshal(message.Payload, unmarshalled)
	if err != nil {
		return
	}
	content := unmarshalled.GetContent()
	head := consensusproto.GetHead(unmarshalled)
	s.syncStatus.HeadsReceive(senderId, s.aclList.Id(), []string{head})
	s.aclList.Lock()
	defer s.aclList.Unlock()
	switch {
	case content.GetHeadUpdate() != nil:
		var syncReq *consensusproto.LogSyncMessage
		syncReq, err = s.syncProtocol.HeadUpdate(ctx, senderId, content.GetHeadUpdate())
		if err != nil || syncReq == nil {
			return
		}
		return s.syncClient.QueueRequest(senderId, syncReq)
	case content.GetFullSyncRequest() != nil:
		return ErrMessageIsRequest
	case content.GetFullSyncResponse() != nil:
		return s.syncProtocol.FullSyncResponse(ctx, senderId, content.GetFullSyncResponse())
	}
	return
}

func (s *syncAclHandler) HandleRequest(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (response *spacesyncproto.ObjectSyncMessage, err error) {
	unmarshalled := &consensusproto.LogSyncMessage{}
	err = proto.Unmarshal(request.Payload, unmarshalled)
	if err != nil {
		return
	}
	fullSyncRequest := unmarshalled.GetContent().GetFullSyncRequest()
	if fullSyncRequest == nil {
		return nil, ErrMessageIsNotRequest
	}
	s.aclList.Lock()
	defer s.aclList.Unlock()
	aclResp, err := s.syncProtocol.FullSyncRequest(ctx, senderId, fullSyncRequest)
	if err != nil {
		return
	}
	return spacesyncproto.MarshallSyncMessage(aclResp, s.spaceId, s.aclList.Id())
}
