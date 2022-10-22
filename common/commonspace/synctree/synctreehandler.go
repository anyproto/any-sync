package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
	"github.com/gogo/protobuf/proto"
)

type syncTreeHandler struct {
	objTree    tree.ObjectTree
	syncClient SyncClient
}

func newSyncTreeHandler(objTree tree.ObjectTree, syncClient SyncClient) synchandler.SyncHandler {
	return &syncTreeHandler{
		objTree:    objTree,
		syncClient: syncClient,
	}
}

func (s *syncTreeHandler) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(msg.Payload, unmarshalled)
	if err != nil {
		return
	}

	content := unmarshalled.GetContent()
	switch {
	case content.GetHeadUpdate() != nil:
		return s.handleHeadUpdate(ctx, senderId, content.GetHeadUpdate(), msg.ReplyId)
	case content.GetFullSyncRequest() != nil:
		return s.handleFullSyncRequest(ctx, senderId, content.GetFullSyncRequest(), msg.ReplyId)
	case content.GetFullSyncResponse() != nil:
		return s.handleFullSyncResponse(ctx, senderId, content.GetFullSyncResponse())
	}
	return nil
}

func (s *syncTreeHandler) handleHeadUpdate(
	ctx context.Context,
	senderId string,
	update *treechangeproto.TreeHeadUpdate,
	replyId string) (err error) {
	log.With("senderId", senderId).
		With("heads", update.Heads).
		With("treeId", s.objTree.ID()).
		Debug("received head update message")
	var (
		fullRequest   *treechangeproto.TreeSyncMessage
		isEmptyUpdate = len(update.Changes) == 0
		objTree       = s.objTree
	)

	err = func() error {
		objTree.Lock()
		defer objTree.Unlock()

		// isEmptyUpdate is sent when the tree is brought up from cache
		if isEmptyUpdate {
			log.With("treeId", objTree.ID()).Debug("is empty update")
			if slice.UnsortedEquals(objTree.Heads(), update.Heads) {
				return nil
			}
			// we need to sync in any case
			fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)
			return err
		}

		if s.alreadyHasHeads(objTree, update.Heads) {
			return nil
		}

		_, err = objTree.AddRawChanges(ctx, update.Changes...)
		if err != nil {
			return err
		}

		if s.alreadyHasHeads(objTree, update.Heads) {
			return nil
		}

		fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)
		return err
	}()

	if fullRequest != nil {
		log.With("senderId", senderId).
			With("heads", fullRequest.GetContent().GetFullSyncRequest().Heads).
			With("treeId", objTree.ID()).
			Debug("sending full sync request")
		return s.syncClient.SendAsync(senderId, fullRequest, replyId)
	}
	log.With("senderId", senderId).
		With("heads", update.Heads).
		With("treeId", objTree.ID()).
		Debug("head update finished correctly")
	return
}

func (s *syncTreeHandler) handleFullSyncRequest(
	ctx context.Context,
	senderId string,
	request *treechangeproto.TreeFullSyncRequest,
	replyId string) (err error) {
	log.With("senderId", senderId).
		With("heads", request.Heads).
		With("treeId", s.objTree.ID()).
		With("trackingId", replyId).
		Debug("received full sync request message")
	var (
		fullResponse *treechangeproto.TreeSyncMessage
		header       = s.objTree.Header()
		objTree      = s.objTree
	)
	defer func() {
		if err != nil {
			s.syncClient.SendAsync(senderId, treechangeproto.WrapError(err, header), replyId)
		}
	}()

	err = func() error {
		objTree.Lock()
		defer objTree.Unlock()

		if header == nil {
			header = objTree.Header()
		}

		if len(request.Changes) != 0 && !s.alreadyHasHeads(objTree, request.Heads) {
			_, err = objTree.AddRawChanges(ctx, request.Changes...)
			if err != nil {
				return err
			}
		}

		fullResponse, err = s.syncClient.CreateFullSyncResponse(objTree, request.Heads, request.SnapshotPath)
		return err
	}()

	if err != nil {
		return
	}
	return s.syncClient.SendAsync(senderId, fullResponse, replyId)
}

func (s *syncTreeHandler) handleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *treechangeproto.TreeFullSyncResponse) (err error) {
	log.With("senderId", senderId).
		With("heads", response.Heads).
		With("treeId", s.objTree.ID()).
		Debug("received full sync response message")
	objTree := s.objTree
	if err != nil {
		log.With("senderId", senderId).
			With("heads", response.Heads).
			With("treeId", s.objTree.ID()).
			Debug("failed to find the tree in full sync response")
		return
	}
	err = func() error {
		objTree.Lock()
		defer objTree.Unlock()

		if s.alreadyHasHeads(objTree, response.Heads) {
			return nil
		}

		_, err = objTree.AddRawChanges(ctx, response.Changes...)
		return err
	}()
	log.With("error", err != nil).
		With("heads", response.Heads).
		With("treeId", s.objTree.ID()).
		Debug("finished full sync response")

	return
}

func (s *syncTreeHandler) alreadyHasHeads(t tree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(t.Heads(), heads) || t.HasChanges(heads...)
}
