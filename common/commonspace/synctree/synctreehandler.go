package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
)

type syncTreeHandler struct {
	objTree    tree.ObjectTree
	syncClient syncservice.SyncClient
}

func newSyncTreeHandler(objTree tree.ObjectTree, syncClient syncservice.SyncClient) synchandler.SyncHandler {
	return &syncTreeHandler{
		objTree:    objTree,
		syncClient: syncClient,
	}
}

func (s *syncTreeHandler) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) error {
	content := msg.GetContent()
	switch {
	case content.GetFullSyncRequest() != nil:
		return s.handleFullSyncRequest(ctx, senderId, content.GetFullSyncRequest(), msg)
	case content.GetFullSyncResponse() != nil:
		return s.handleFullSyncResponse(ctx, senderId, content.GetFullSyncResponse(), msg)
	case content.GetHeadUpdate() != nil:
		return s.handleHeadUpdate(ctx, senderId, content.GetHeadUpdate(), msg)
	}
	return nil
}

func (s *syncTreeHandler) handleHeadUpdate(
	ctx context.Context,
	senderId string,
	update *spacesyncproto.ObjectHeadUpdate,
	msg *spacesyncproto.ObjectSyncMessage) (err error) {
	log.With("senderId", senderId).
		With("heads", update.Heads).
		With("treeId", msg.TreeId).
		Debug("received head update message")
	var (
		fullRequest   *spacesyncproto.ObjectSyncMessage
		isEmptyUpdate = len(update.Changes) == 0
		objTree       = s.objTree
	)

	err = func() error {
		objTree.Lock()
		defer objTree.Unlock()

		// isEmptyUpdate is sent when the tree is brought up from cache
		if isEmptyUpdate {
			log.With("treeId", msg.TreeId).Debug("is empty update")
			if slice.UnsortedEquals(objTree.Heads(), update.Heads) {
				return nil
			}
			// we need to sync in any case
			fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath, msg.TrackingId)
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

		fullRequest, err = s.syncClient.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath, msg.TrackingId)
		return err
	}()

	if fullRequest != nil {
		log.With("senderId", senderId).
			With("heads", fullRequest.GetContent().GetFullSyncRequest().Heads).
			With("treeId", msg.TreeId).
			Debug("sending full sync request")
		return s.syncClient.SendAsync([]string{senderId}, fullRequest)
	}
	log.With("senderId", senderId).
		With("heads", update.Heads).
		With("treeId", msg.TreeId).
		Debug("head update finished correctly")
	return
}

func (s *syncTreeHandler) handleFullSyncRequest(
	ctx context.Context,
	senderId string,
	request *spacesyncproto.ObjectFullSyncRequest,
	msg *spacesyncproto.ObjectSyncMessage) (err error) {
	log.With("senderId", senderId).
		With("heads", request.Heads).
		With("treeId", msg.TreeId).
		With("trackingId", msg.TrackingId).
		Debug("received full sync request message")
	var (
		fullResponse *spacesyncproto.ObjectSyncMessage
		header       = msg.RootChange
		objTree      = s.objTree
	)
	defer func() {
		if err != nil {
			s.syncClient.SendAsync([]string{senderId}, spacesyncproto.WrapError(err, header, msg.TreeId, msg.TrackingId))
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

		fullResponse, err = s.syncClient.CreateFullSyncResponse(objTree, request.Heads, request.SnapshotPath, msg.TrackingId)
		return err
	}()

	if err != nil {
		return
	}
	return s.syncClient.SendAsync([]string{senderId}, fullResponse)
}

func (s *syncTreeHandler) handleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *spacesyncproto.ObjectFullSyncResponse,
	msg *spacesyncproto.ObjectSyncMessage) (err error) {
	log.With("senderId", senderId).
		With("heads", response.Heads).
		With("treeId", msg.TreeId).
		Debug("received full sync response message")
	objTree := s.objTree
	if err != nil {
		log.With("senderId", senderId).
			With("heads", response.Heads).
			With("treeId", msg.TreeId).
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
		With("treeId", msg.TreeId).
		Debug("finished full sync response")

	return
}

func (s *syncTreeHandler) alreadyHasHeads(t tree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(t.Heads(), heads) || t.HasChanges(heads...)
}
