package synctree

import (
	"context"
	"errors"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/gogo/protobuf/proto"
	"sync"
)

var (
	ErrMessageIsRequest    = errors.New("message is request")
	ErrMessageIsNotRequest = errors.New("message is not request")
)

type syncTreeHandler struct {
	objTree      objecttree.ObjectTree
	syncClient   SyncClient
	syncProtocol TreeSyncProtocol
	syncStatus   syncstatus.StatusUpdater
	handlerLock  sync.Mutex
	spaceId      string
	queue        ReceiveQueue
}

const maxQueueSize = 5

func newSyncTreeHandler(spaceId string, objTree objecttree.ObjectTree, syncClient SyncClient, syncStatus syncstatus.StatusUpdater) synchandler.SyncHandler {
	return &syncTreeHandler{
		objTree:      objTree,
		syncProtocol: newTreeSyncProtocol(spaceId, objTree, syncClient),
		syncClient:   syncClient,
		syncStatus:   syncStatus,
		spaceId:      spaceId,
		queue:        newReceiveQueue(maxQueueSize),
	}
}

func (s *syncTreeHandler) HandleRequest(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (response *spacesyncproto.ObjectSyncMessage, err error) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(request.Payload, unmarshalled)
	if err != nil {
		return
	}
	fullSyncRequest := unmarshalled.GetContent().GetFullSyncRequest()
	if fullSyncRequest == nil {
		err = ErrMessageIsNotRequest
		return
	}
	s.syncStatus.HeadsReceive(senderId, request.ObjectId, treechangeproto.GetHeads(unmarshalled))
	s.objTree.Lock()
	defer s.objTree.Unlock()
	treeResp, err := s.syncProtocol.FullSyncRequest(ctx, senderId, fullSyncRequest)
	if err != nil {
		return
	}
	response, err = MarshallTreeMessage(treeResp, s.spaceId, request.ObjectId, "")
	return
}

func (s *syncTreeHandler) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(msg.Payload, unmarshalled)
	if err != nil {
		return
	}
	s.syncStatus.HeadsReceive(senderId, msg.ObjectId, treechangeproto.GetHeads(unmarshalled))

	queueFull := s.queue.AddMessage(senderId, unmarshalled, msg.RequestId)
	if queueFull {
		return
	}

	return s.handleMessage(ctx, senderId)
}

func (s *syncTreeHandler) handleMessage(ctx context.Context, senderId string) (err error) {
	s.objTree.Lock()
	defer s.objTree.Unlock()
	msg, _, err := s.queue.GetMessage(senderId)
	if err != nil {
		return
	}

	defer s.queue.ClearQueue(senderId)

	treeId := s.objTree.Id()
	content := msg.GetContent()
	switch {
	case content.GetHeadUpdate() != nil:
		var syncReq *treechangeproto.TreeSyncMessage
		syncReq, err = s.syncProtocol.HeadUpdate(ctx, senderId, content.GetHeadUpdate())
		if err != nil || syncReq == nil {
			return
		}
		return s.syncClient.QueueRequest(senderId, treeId, syncReq)
	case content.GetFullSyncRequest() != nil:
		return ErrMessageIsRequest
	case content.GetFullSyncResponse() != nil:
		return s.syncProtocol.FullSyncResponse(ctx, senderId, content.GetFullSyncResponse())
	}
	return
}
