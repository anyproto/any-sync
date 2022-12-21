package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/statusservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync"
)

type syncTreeHandler struct {
	objTree       tree.ObjectTree
	syncClient    SyncClient
	statusService statusservice.StatusService
	handlerLock   sync.Mutex
	queue         ReceiveQueue
}

const maxQueueSize = 5

func newSyncTreeHandler(objTree tree.ObjectTree, syncClient SyncClient, statusService statusservice.StatusService) synchandler.SyncHandler {
	return &syncTreeHandler{
		objTree:       objTree,
		syncClient:    syncClient,
		statusService: statusService,
		queue:         newReceiveQueue(maxQueueSize),
	}
}

func (s *syncTreeHandler) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	// TODO: when implementing sync status check msg heads before sending into queue
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(msg.Payload, unmarshalled)
	if err != nil {
		return
	}

	s.statusService.HeadsReceive(senderId, msg.ObjectId, treechangeproto.GetHeads(unmarshalled))

	queueFull := s.queue.AddMessage(senderId, unmarshalled, msg.ReplyId)
	if queueFull {
		return
	}

	return s.handleMessage(ctx, senderId)
}

func (s *syncTreeHandler) handleMessage(ctx context.Context, senderId string) (err error) {
	s.objTree.Lock()
	defer s.objTree.Unlock()
	msg, replyId, err := s.queue.GetMessage(senderId)
	if err != nil {
		return
	}

	defer s.queue.ClearQueue(senderId)

	content := msg.GetContent()
	switch {
	case content.GetHeadUpdate() != nil:
		return s.handleHeadUpdate(ctx, senderId, content.GetHeadUpdate(), replyId)
	case content.GetFullSyncRequest() != nil:
		return s.handleFullSyncRequest(ctx, senderId, content.GetFullSyncRequest(), replyId)
	case content.GetFullSyncResponse() != nil:
		return s.handleFullSyncResponse(ctx, senderId, content.GetFullSyncResponse())
	}
	return
}

func (s *syncTreeHandler) handleHeadUpdate(
	ctx context.Context,
	senderId string,
	update *treechangeproto.TreeHeadUpdate,
	replyId string) (err error) {
	var (
		fullRequest   *treechangeproto.TreeSyncMessage
		isEmptyUpdate = len(update.Changes) == 0
		objTree       = s.objTree
	)

	log := log.With("senderId", senderId).
		With("heads", objTree.Heads()).
		With("treeId", objTree.ID())
	log.Debug("received head update message")

	defer func() {
		if err != nil {
			log.With(zap.Error(err)).Debug("head update finished with error")
		} else if fullRequest != nil {
			log.Debug("sending full sync request")
		} else {
			if !isEmptyUpdate {
				log.Debug("head update finished correctly")
			}
		}
	}()

	// isEmptyUpdate is sent when the tree is brought up from cache
	if isEmptyUpdate {
		log.With("treeId", objTree.ID()).Debug("is empty update")
		if slice.UnsortedEquals(objTree.Heads(), update.Heads) {
			return
		}
		// we need to sync in any case
		fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)
		if err != nil {
			return
		}

		return s.syncClient.SendAsync(senderId, fullRequest, replyId)
	}

	if s.alreadyHasHeads(objTree, update.Heads) {
		return
	}

	_, err = objTree.AddRawChanges(ctx, tree.RawChangesPayload{
		NewHeads:   update.Heads,
		RawChanges: update.Changes,
	})
	if err != nil {
		return
	}

	if s.alreadyHasHeads(objTree, update.Heads) {
		return
	}

	fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)
	if err != nil {
		return
	}

	return s.syncClient.SendAsync(senderId, fullRequest, replyId)
}

func (s *syncTreeHandler) handleFullSyncRequest(
	ctx context.Context,
	senderId string,
	request *treechangeproto.TreeFullSyncRequest,
	replyId string) (err error) {
	var (
		fullResponse *treechangeproto.TreeSyncMessage
		header       = s.objTree.Header()
		objTree      = s.objTree
	)

	log := log.With("senderId", senderId).
		With("heads", request.Heads).
		With("treeId", s.objTree.ID()).
		With("replyId", replyId)
	log.Debug("received full sync request message")

	defer func() {
		if err != nil {
			log.With(zap.Error(err)).Debug("full sync request finished with error")

			s.syncClient.SendAsync(senderId, treechangeproto.WrapError(err, header), replyId)
			return
		} else if fullResponse != nil {
			log.Debug("full sync response sent")
		}
	}()

	if len(request.Changes) != 0 && !s.alreadyHasHeads(objTree, request.Heads) {
		_, err = objTree.AddRawChanges(ctx, tree.RawChangesPayload{
			NewHeads:   request.Heads,
			RawChanges: request.Changes,
		})
		if err != nil {
			return
		}
	}
	fullResponse, err = s.syncClient.CreateFullSyncResponse(objTree, request.Heads, request.SnapshotPath)
	if err != nil {
		return
	}

	return s.syncClient.SendAsync(senderId, fullResponse, replyId)
}

func (s *syncTreeHandler) handleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *treechangeproto.TreeFullSyncResponse) (err error) {
	var (
		objTree = s.objTree
	)
	log := log.With("senderId", senderId).
		With("heads", response.Heads).
		With("treeId", s.objTree.ID())
	log.Debug("received full sync response message")

	defer func() {
		if err != nil {
			log.With(zap.Error(err)).Debug("full sync response failed")
		} else {
			log.Debug("full sync response succeeded")
		}
	}()

	if s.alreadyHasHeads(objTree, response.Heads) {
		return
	}

	_, err = objTree.AddRawChanges(ctx, tree.RawChangesPayload{
		NewHeads:   response.Heads,
		RawChanges: response.Changes,
	})
	return
}

func (s *syncTreeHandler) alreadyHasHeads(t tree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(t.Heads(), heads) || t.HasChanges(heads...)
}
