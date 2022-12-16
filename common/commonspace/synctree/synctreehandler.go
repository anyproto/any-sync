package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"sync"
)

type syncTreeHandler struct {
	objTree     tree.ObjectTree
	syncClient  SyncClient
	handlerLock sync.Mutex
	queue       ReceiveQueue
}

const maxQueueSize = 5

type treeMsg struct {
	replyId     string
	syncMessage *treechangeproto.TreeSyncMessage
}

func newSyncTreeHandler(objTree tree.ObjectTree, syncClient SyncClient) synchandler.SyncHandler {
	return &syncTreeHandler{
		objTree:    objTree,
		syncClient: syncClient,
		queue:      newReceiveQueue(maxQueueSize),
	}
}

type sendFunc = func() error

func (s *syncTreeHandler) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(msg.Payload, unmarshalled)
	if err != nil {
		return
	}

	queueFull := s.queue.AddMessage(senderId, treeMsg{msg.ReplyId, unmarshalled})
	if queueFull {
		return
	}

	actions, err := s.handleMessage(ctx, senderId)
	if err != nil {
		log.With(zap.Error(err)).Debug("handling message finished with error")
	}
	for _, action := range actions {
		err := action()
		if err != nil {
			log.With(zap.Error(err)).Debug("error while sending action")
		}
	}

	return
}

func (s *syncTreeHandler) handleMessage(ctx context.Context, senderId string) (actions []sendFunc, err error) {
	s.objTree.Lock()
	defer s.objTree.Unlock()
	msg, err := s.queue.GetMessage(senderId)
	if err != nil {
		return
	}

	defer s.queue.ClearQueue(senderId)

	content := msg.syncMessage.GetContent()
	switch {
	case content.GetHeadUpdate() != nil:
		return s.handleHeadUpdate(ctx, senderId, content.GetHeadUpdate(), msg.replyId)
	case content.GetFullSyncRequest() != nil:
		return s.handleFullSyncRequest(ctx, senderId, content.GetFullSyncRequest(), msg.replyId)
	case content.GetFullSyncResponse() != nil:
		return s.handleFullSyncResponse(ctx, senderId, content.GetFullSyncResponse())
	}
	return
}

func (s *syncTreeHandler) handleHeadUpdate(
	ctx context.Context,
	senderId string,
	update *treechangeproto.TreeHeadUpdate,
	replyId string) (actions []sendFunc, err error) {
	log.With("senderId", senderId).
		With("heads", update.Heads).
		With("treeId", s.objTree.ID()).
		Debug("received head update message")
	var (
		fullRequest   *treechangeproto.TreeSyncMessage
		headUpdate    *treechangeproto.TreeSyncMessage
		isEmptyUpdate = len(update.Changes) == 0
		objTree       = s.objTree
		addResult     tree.AddResult
	)
	defer func() {
		if headUpdate != nil {
			actions = append(actions, func() error {
				return s.syncClient.BroadcastAsync(headUpdate)
			})
		}
		if fullRequest != nil {
			actions = append(actions, func() error {
				return s.syncClient.SendAsync(senderId, fullRequest, replyId)
			})
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
		return
	}

	if s.alreadyHasHeads(objTree, update.Heads) {
		return
	}

	addResult, err = objTree.AddRawChanges(ctx, tree.RawChangesPayload{
		NewHeads:   update.Heads,
		RawChanges: update.Changes,
	})
	if err != nil {
		return
	}
	if addResult.Mode != tree.Nothing {
		headUpdate = s.syncClient.CreateHeadUpdate(objTree, addResult.Added)
	}

	if s.alreadyHasHeads(objTree, update.Heads) {
		return
	}

	fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)

	if fullRequest != nil {
		log.With("senderId", senderId).
			With("heads", objTree.Heads()).
			With("treeId", objTree.ID()).
			Debug("sending full sync request")
	} else {
		log.With("senderId", senderId).
			With("heads", update.Heads).
			With("treeId", objTree.ID()).
			Debug("head update finished correctly")
	}
	return
}

func (s *syncTreeHandler) handleFullSyncRequest(
	ctx context.Context,
	senderId string,
	request *treechangeproto.TreeFullSyncRequest,
	replyId string) (actions []sendFunc, err error) {
	log.With("senderId", senderId).
		With("heads", request.Heads).
		With("treeId", s.objTree.ID()).
		With("trackingId", replyId).
		Debug("received full sync request message")
	var (
		fullResponse *treechangeproto.TreeSyncMessage
		headUpdate   *treechangeproto.TreeSyncMessage
		addResult    tree.AddResult
		header       = s.objTree.Header()
		objTree      = s.objTree
	)
	defer func() {
		if err != nil {
			actions = append(actions, func() error {
				return s.syncClient.SendAsync(senderId, treechangeproto.WrapError(err, header), replyId)
			})
			return
		}

		if headUpdate != nil {
			actions = append(actions, func() error {
				return s.syncClient.BroadcastAsync(headUpdate)
			})
		}
		if fullResponse != nil {
			actions = append(actions, func() error {
				return s.syncClient.SendAsync(senderId, fullResponse, replyId)
			})
		}
	}()

	if len(request.Changes) != 0 && !s.alreadyHasHeads(objTree, request.Heads) {
		addResult, err = objTree.AddRawChanges(ctx, tree.RawChangesPayload{
			NewHeads:   request.Heads,
			RawChanges: request.Changes,
		})
		if err != nil {
			return
		}
		if addResult.Mode != tree.Nothing {
			headUpdate = s.syncClient.CreateHeadUpdate(objTree, addResult.Added)
		}
	}
	fullResponse, err = s.syncClient.CreateFullSyncResponse(objTree, request.Heads, request.SnapshotPath)
	return
}

func (s *syncTreeHandler) handleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *treechangeproto.TreeFullSyncResponse) (actions []sendFunc, err error) {
	log.With("senderId", senderId).
		With("heads", response.Heads).
		With("treeId", s.objTree.ID()).
		Debug("received full sync response message")
	var (
		objTree    = s.objTree
		addResult  tree.AddResult
		headUpdate *treechangeproto.TreeSyncMessage
	)
	defer func() {
		if headUpdate != nil {
			actions = append(actions, func() error {
				return s.syncClient.BroadcastAsync(headUpdate)
			})
		}
	}()

	if s.alreadyHasHeads(objTree, response.Heads) {
		return
	}

	addResult, err = objTree.AddRawChanges(ctx, tree.RawChangesPayload{
		NewHeads:   response.Heads,
		RawChanges: response.Changes,
	})
	if err != nil {
		return
	}
	if addResult.Mode != tree.Nothing {
		headUpdate = s.syncClient.CreateHeadUpdate(objTree, addResult.Added)
	}

	log.With("error", err != nil).
		With("heads", response.Heads).
		With("treeId", s.objTree.ID()).
		Debug("finished full sync response")
	return
}

func (s *syncTreeHandler) alreadyHasHeads(t tree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(t.Heads(), heads) || t.HasChanges(heads...)
}
