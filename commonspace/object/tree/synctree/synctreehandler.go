package synctree

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/util/slice"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync"
)

type syncTreeHandler struct {
	objTree     objecttree.ObjectTree
	syncClient  objectsync.SyncClient
	syncStatus  syncstatus.StatusUpdater
	handlerLock sync.Mutex
	spaceId     string
	queue       ReceiveQueue
}

const maxQueueSize = 5

func newSyncTreeHandler(spaceId string, objTree objecttree.ObjectTree, syncClient objectsync.SyncClient, syncStatus syncstatus.StatusUpdater) synchandler.SyncHandler {
	return &syncTreeHandler{
		objTree:    objTree,
		syncClient: syncClient,
		syncStatus: syncStatus,
		spaceId:    spaceId,
		queue:      newReceiveQueue(maxQueueSize),
	}
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
	log := log.With(
		zap.Strings("update heads", update.Heads),
		zap.String("treeId", objTree.Id()),
		zap.String("spaceId", s.spaceId),
		zap.Int("len(update changes)", len(update.Changes)))
	log.DebugCtx(ctx, "received head update message")

	defer func() {
		if err != nil {
			log.With(zap.Error(err)).Debug("head update finished with error")
		} else if fullRequest != nil {
			cnt := fullRequest.Content.GetFullSyncRequest()
			log = log.With(zap.Strings("request heads", cnt.Heads), zap.Int("len(request changes)", len(cnt.Changes)))
			log.DebugCtx(ctx, "sending full sync request")
		} else {
			if !isEmptyUpdate {
				log.DebugCtx(ctx, "head update finished correctly")
			}
		}
	}()

	// isEmptyUpdate is sent when the tree is brought up from cache
	if isEmptyUpdate {
		headEquals := slice.UnsortedEquals(objTree.Heads(), update.Heads)
		log.DebugCtx(ctx, "is empty update", zap.String("treeId", objTree.Id()), zap.Bool("headEquals", headEquals))
		if headEquals {
			return
		}

		// we need to sync in any case
		fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)
		if err != nil {
			return
		}

		return s.syncClient.SendWithReply(ctx, senderId, fullRequest, replyId)
	}

	if s.alreadyHasHeads(objTree, update.Heads) {
		return
	}

	_, err = objTree.AddRawChanges(ctx, objecttree.RawChangesPayload{
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

	return s.syncClient.SendWithReply(ctx, senderId, fullRequest, replyId)
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

	log := log.With(zap.String("senderId", senderId),
		zap.Strings("request heads", request.Heads),
		zap.String("treeId", s.objTree.Id()),
		zap.String("replyId", replyId),
		zap.String("spaceId", s.spaceId),
		zap.Int("len(request changes)", len(request.Changes)))
	log.DebugCtx(ctx, "received full sync request message")

	defer func() {
		if err != nil {
			log.With(zap.Error(err)).DebugCtx(ctx, "full sync request finished with error")
			s.syncClient.SendWithReply(ctx, senderId, treechangeproto.WrapError(treechangeproto.ErrFullSync, header), replyId)
			return
		} else if fullResponse != nil {
			cnt := fullResponse.Content.GetFullSyncResponse()
			log = log.With(zap.Strings("response heads", cnt.Heads), zap.Int("len(response changes)", len(cnt.Changes)))
			log.DebugCtx(ctx, "full sync response sent")
		}
	}()

	if len(request.Changes) != 0 && !s.alreadyHasHeads(objTree, request.Heads) {
		_, err = objTree.AddRawChanges(ctx, objecttree.RawChangesPayload{
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

	return s.syncClient.SendWithReply(ctx, senderId, fullResponse, replyId)
}

func (s *syncTreeHandler) handleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *treechangeproto.TreeFullSyncResponse) (err error) {
	var (
		objTree = s.objTree
	)
	log := log.With(
		zap.Strings("heads", response.Heads),
		zap.String("treeId", s.objTree.Id()),
		zap.String("spaceId", s.spaceId),
		zap.Int("len(changes)", len(response.Changes)))
	log.DebugCtx(ctx, "received full sync response message")

	defer func() {
		if err != nil {
			log.With(zap.Error(err)).DebugCtx(ctx, "full sync response failed")
		} else {
			log.DebugCtx(ctx, "full sync response succeeded")
		}
	}()

	if s.alreadyHasHeads(objTree, response.Heads) {
		return
	}

	_, err = objTree.AddRawChanges(ctx, objecttree.RawChangesPayload{
		NewHeads:   response.Heads,
		RawChanges: response.Changes,
	})
	return
}

func (s *syncTreeHandler) alreadyHasHeads(t objecttree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(t.Heads(), heads) || t.HasChanges(heads...)
}
