package synctree

import (
	"context"
	"errors"
	"sync"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/util/slice"
	"github.com/gogo/protobuf/proto"
)

var (
	ErrMessageIsRequest    = errors.New("message is request")
	ErrMessageIsNotRequest = errors.New("message is not request")
	ErrMoreThanOneRequest  = errors.New("more than one request for same peer")
)

type syncTreeHandler struct {
	objTree      objecttree.ObjectTree
	syncClient   SyncClient
	syncProtocol TreeSyncProtocol
	syncStatus   syncstatus.StatusUpdater
	spaceId      string

	handlerLock     sync.Mutex
	pendingRequests map[string]struct{}
	heads           []string
}

const maxQueueSize = 5

func newSyncTreeHandler(spaceId string, objTree objecttree.ObjectTree, syncClient SyncClient, syncStatus syncstatus.StatusUpdater) synchandler.SyncHandler {
	return &syncTreeHandler{
		objTree:         objTree,
		syncProtocol:    newTreeSyncProtocol(spaceId, objTree, syncClient),
		syncClient:      syncClient,
		syncStatus:      syncStatus,
		spaceId:         spaceId,
		pendingRequests: make(map[string]struct{}),
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
		return nil, ErrMessageIsNotRequest
	}
	// setting pending requests
	s.handlerLock.Lock()
	_, exists := s.pendingRequests[senderId]
	if exists {
		s.handlerLock.Unlock()
		return nil, ErrMoreThanOneRequest
	}
	s.pendingRequests[senderId] = struct{}{}
	s.handlerLock.Unlock()

	response, err = s.handleRequest(ctx, senderId, fullSyncRequest)

	// removing pending requests
	s.handlerLock.Lock()
	delete(s.pendingRequests, senderId)
	s.handlerLock.Unlock()
	return
}

func (s *syncTreeHandler) handleRequest(ctx context.Context, senderId string, fullSyncRequest *treechangeproto.TreeFullSyncRequest) (response *spacesyncproto.ObjectSyncMessage, err error) {
	s.objTree.Lock()
	defer s.objTree.Unlock()
	treeResp, err := s.syncProtocol.FullSyncRequest(ctx, senderId, fullSyncRequest)
	if err != nil {
		return
	}
	response, err = MarshallTreeMessage(treeResp, s.spaceId, s.objTree.Id(), "")
	return
}

func (s *syncTreeHandler) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(msg.Payload, unmarshalled)
	if err != nil {
		return
	}
	heads := treechangeproto.GetHeads(unmarshalled)
	s.syncStatus.HeadsReceive(senderId, msg.ObjectId, heads)
	s.handlerLock.Lock()
	// if the update has same heads then returning not to hang on a lock
	if unmarshalled.GetContent().GetHeadUpdate() != nil && slice.UnsortedEquals(heads, s.heads) {
		s.handlerLock.Unlock()
		return
	}
	s.handlerLock.Unlock()
	return s.handleMessage(ctx, unmarshalled, senderId)
}

func (s *syncTreeHandler) handleMessage(ctx context.Context, msg *treechangeproto.TreeSyncMessage, senderId string) (err error) {
	s.objTree.Lock()
	defer s.objTree.Unlock()
	var (
		copyHeads = make([]string, 0, len(s.objTree.Heads()))
		treeId    = s.objTree.Id()
		content   = msg.GetContent()
	)

	// getting old heads
	copyHeads = append(copyHeads, s.objTree.Heads()...)
	defer func() {
		// checking if something changed
		if !slice.UnsortedEquals(copyHeads, s.objTree.Heads()) {
			s.handlerLock.Lock()
			defer s.handlerLock.Unlock()
			s.heads = s.heads[:0]
			for _, h := range s.objTree.Heads() {
				s.heads = append(s.heads, h)
			}
		}
	}()

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
