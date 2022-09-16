package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

type syncHandler struct {
	treeCache  cache.TreeCache
	syncClient SyncClient
}

type SyncHandler interface {
	HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error)
}

func newSyncHandler(treeCache cache.TreeCache, syncClient SyncClient) *syncHandler {
	return &syncHandler{
		treeCache:  treeCache,
		syncClient: syncClient,
	}
}

func (s *syncHandler) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) error {
	msg := message.GetContent()
	switch {
	case msg.GetFullSyncRequest() != nil:
		return s.HandleFullSyncRequest(ctx, senderId, msg.GetFullSyncRequest(), message.GetTreeHeader(), message.GetTreeId())
	case msg.GetFullSyncResponse() != nil:
		return s.HandleFullSyncResponse(ctx, senderId, msg.GetFullSyncResponse(), message.GetTreeHeader(), message.GetTreeId())
	case msg.GetHeadUpdate() != nil:
		return s.HandleHeadUpdate(ctx, senderId, msg.GetHeadUpdate(), message.GetTreeHeader(), message.GetTreeId())
	}
	return nil
}

func (s *syncHandler) HandleHeadUpdate(
	ctx context.Context,
	senderId string,
	update *spacesyncproto.ObjectHeadUpdate,
	header *aclpb.TreeHeader,
	treeId string) (err error) {

	var (
		fullRequest *spacesyncproto.ObjectFullSyncRequest
		result      tree.AddResult
	)

	res, err := s.treeCache.GetTree(ctx, treeId)
	if err != nil {
		return
	}

	err = func() error {
		objTree := res.TreeContainer.Tree()
		objTree.Lock()
		defer res.Release()
		defer objTree.Unlock()

		if slice.UnsortedEquals(update.Heads, objTree.Heads()) {
			return nil
		}

		result, err = objTree.AddRawChanges(ctx, update.Changes...)
		if err != nil {
			return err
		}

		// if we couldn't add all the changes
		if len(update.Changes) != len(result.Added) {
			fullRequest, err = s.prepareFullSyncRequest(objTree, update)
			if err != nil {
				return err
			}
		}
		return nil
	}()

	if fullRequest != nil {
		return s.syncClient.SendAsync(senderId, spacesyncproto.WrapFullRequest(fullRequest, header, treeId))
	}
	return
}

func (s *syncHandler) HandleFullSyncRequest(
	ctx context.Context,
	senderId string,
	request *spacesyncproto.ObjectFullSyncRequest,
	header *aclpb.TreeHeader,
	treeId string) (err error) {

	var fullResponse *spacesyncproto.ObjectFullSyncResponse

	res, err := s.treeCache.GetTree(ctx, treeId)
	if err != nil {
		return
	}

	err = func() error {
		objTree := res.TreeContainer.Tree()
		objTree.Lock()
		defer res.Release()
		defer objTree.Unlock()

		_, err = objTree.AddRawChanges(ctx, request.Changes...)
		if err != nil {
			return err
		}

		fullResponse, err = s.prepareFullSyncResponse(treeId, request.SnapshotPath, request.Heads, objTree)
		return err
	}()

	if err != nil {
		return
	}
	return s.syncClient.SendAsync(senderId, spacesyncproto.WrapFullResponse(fullResponse, header, treeId))
}

func (s *syncHandler) HandleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *spacesyncproto.ObjectFullSyncResponse,
	header *aclpb.TreeHeader,
	treeId string) (err error) {

	res, err := s.treeCache.GetTree(ctx, treeId)
	if err != nil {
		return
	}

	err = func() error {
		objTree := res.TreeContainer.Tree()
		objTree.Lock()
		defer res.Release()
		defer objTree.Unlock()

		// if we already have the heads for whatever reason
		if slice.UnsortedEquals(response.Heads, objTree.Heads()) {
			return nil
		}

		_, err = objTree.AddRawChanges(ctx, response.Changes...)
		return err
	}()

	return
}

func (s *syncHandler) prepareFullSyncRequest(
	t tree.ObjectTree,
	update *spacesyncproto.ObjectHeadUpdate) (req *spacesyncproto.ObjectFullSyncRequest, err error) {
	req = &spacesyncproto.ObjectFullSyncRequest{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	}
	if len(update.Changes) != 0 {
		var changesAfterSnapshot []*aclpb.RawTreeChangeWithId
		changesAfterSnapshot, err = t.ChangesAfterCommonSnapshot(update.SnapshotPath, update.Heads)
		if err != nil {
			return
		}
		req.Changes = changesAfterSnapshot
	}
	return &spacesyncproto.ObjectFullSyncRequest{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	}, nil
}

func (s *syncHandler) prepareFullSyncResponse(
	treeId string,
	theirPath,
	theirHeads []string,
	t tree.ObjectTree) (*spacesyncproto.ObjectFullSyncResponse, error) {
	ourChanges, err := t.ChangesAfterCommonSnapshot(theirPath, theirHeads)
	if err != nil {
		return nil, err
	}

	return &spacesyncproto.ObjectFullSyncResponse{
		Heads:        t.Heads(),
		Changes:      ourChanges,
		SnapshotPath: t.SnapshotPath(),
	}, nil
}
