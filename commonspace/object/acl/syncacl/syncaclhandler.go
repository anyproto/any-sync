package syncacl

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/peer"
)

var (
	ErrUnexpectedMessageType  = errors.New("unexpected message type")
	ErrUnexpectedResponseType = errors.New("unexpected response type")
	ErrUnexpectedRequestType  = errors.New("unexpected request type")
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

func (s *syncAclHandler) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (syncdeps.Request, error) {
	update, ok := headUpdate.(*objectsync.HeadUpdate)
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

func (s *syncAclHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, send func(resp proto.Message) error) (syncdeps.Request, error) {
	req, ok := rq.(*Request)
	if !ok {
		return nil, ErrUnexpectedRequestType
	}
	s.aclList.Lock()
	if !s.aclList.HasHead(req.head) {
		s.aclList.Unlock()
		return s.syncClient.CreateFullSyncRequest(req.peerId, s.aclList), nil
	}
	resp, err := s.syncClient.CreateFullSyncResponse(s.aclList, req.head)
	if err != nil {
		s.aclList.Unlock()
		return nil, err
	}
	s.aclList.Unlock()
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
	s.aclList.Lock()
	defer s.aclList.Unlock()
	return s.aclList.AddRawRecords(response.records)
}

func (s *syncAclHandler) ResponseCollector() syncdeps.ResponseCollector {
	return newResponseCollector(s)
}
