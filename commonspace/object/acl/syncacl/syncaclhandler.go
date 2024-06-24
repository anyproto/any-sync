package syncacl

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/peer"
)

var (
	ErrUnexpectedMessageType  = errors.New("unexpected message type")
	ErrUnexpectedResponseType = errors.New("unexpected response type")
	ErrUnexpectedRequestType  = errors.New("unexpected request type")
	ErrUnknownHead            = errors.New("unknown head")
)

var (
	ErrMessageIsRequest    = errors.New("message is request")
	ErrMessageIsNotRequest = errors.New("message is not request")
)

type syncAclHandler struct {
	aclList    list.AclList
	syncClient SyncClient
	spaceId    string
}

func newSyncAclHandler(spaceId string, aclList list.AclList, syncClient SyncClient) syncdeps.ObjectSyncHandler {
	return &syncAclHandler{
		spaceId:    spaceId,
		aclList:    aclList,
		syncClient: syncClient,
	}
}

func (s *syncAclHandler) HandleHeadUpdate(ctx context.Context, statusUpdater syncstatus.StatusUpdater, headUpdate drpc.Message) (syncdeps.Request, error) {
	update, ok := headUpdate.(*objectmessages.HeadUpdate)
	if !ok {
		return nil, ErrUnexpectedResponseType
	}
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	objMsg := &consensusproto.LogSyncMessage{}
	err = proto.Unmarshal(update.Bytes, objMsg)
	if err != nil {
		return nil, err
	}
	if objMsg.GetContent().GetHeadUpdate() == nil {
		return nil, ErrUnexpectedMessageType
	}
	contentUpdate := objMsg.GetContent().GetHeadUpdate()
	statusUpdater.HeadsReceive(peerId, update.ObjectId(), []string{contentUpdate.Head})
	s.aclList.Lock()
	defer s.aclList.Unlock()
	if len(contentUpdate.Records) == 0 {
		if s.aclList.HasHead(contentUpdate.Head) {
			return nil, nil
		}
		return s.syncClient.CreateFullSyncRequest(peerId, s.aclList), nil
	}
	err = s.aclList.AddRawRecords(contentUpdate.Records)
	if !errors.Is(err, list.ErrIncorrectRecordSequence) {
		return nil, err
	}
	return s.syncClient.CreateFullSyncRequest(peerId, s.aclList), nil
}

func (s *syncAclHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, updater syncdeps.QueueSizeUpdater, send func(resp proto.Message) error) (syncdeps.Request, error) {
	req, ok := rq.(*objectmessages.Request)
	if !ok {
		return nil, ErrUnexpectedRequestType
	}
	syncMsg := &consensusproto.LogSyncMessage{}
	err := proto.Unmarshal(req.Bytes, syncMsg)
	if err != nil {
		return nil, err
	}
	request := syncMsg.GetContent().GetFullSyncRequest()
	if request == nil {
		return nil, ErrUnexpectedRequestType
	}
	s.aclList.Lock()
	if !s.aclList.HasHead(request.Head) {
		s.aclList.Unlock()
		return s.syncClient.CreateFullSyncRequest(req.PeerId(), s.aclList), ErrUnknownHead
	}
	resp, err := s.syncClient.CreateFullSyncResponse(s.aclList, request.Head)
	if err != nil {
		s.aclList.Unlock()
		return nil, err
	}
	s.aclList.Unlock()
	size := resp.MsgSize()
	updater.UpdateQueueSize(size, syncdeps.MsgTypeSentResponse, true)
	defer updater.UpdateQueueSize(size, syncdeps.MsgTypeSentResponse, false)
	protoMsg, err := resp.ProtoMessage()
	if err != nil {
		return nil, err
	}
	return nil, send(protoMsg)
}

func (s *syncAclHandler) HandleResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	response, ok := resp.(*Response)
	if !ok {
		return ErrUnexpectedResponseType
	}
	if len(response.records) == 0 {
		return nil
	}
	s.aclList.Lock()
	defer s.aclList.Unlock()
	return s.aclList.AddRawRecords(response.records)
}

func (s *syncAclHandler) ResponseCollector() syncdeps.ResponseCollector {
	return newResponseCollector(s)
}
