//go:generate mockgen -destination mock_syncservice/mock_syncservice.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice SyncClient
package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

type syncHandler struct {
	spaceId    string
	treeCache  cache.TreeCache
	syncClient SyncClient
	factory    RequestFactory
}

type SyncHandler interface {
	HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error)
}

func newSyncHandler(spaceId string, treeCache cache.TreeCache, syncClient SyncClient, factory RequestFactory) *syncHandler {
	return &syncHandler{
		spaceId:    spaceId,
		treeCache:  treeCache,
		syncClient: syncClient,
		factory:    factory,
	}
}

func (s *syncHandler) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) error {
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

func (s *syncHandler) handleHeadUpdate(
	ctx context.Context,
	senderId string,
	update *spacesyncproto.ObjectHeadUpdate,
	msg *spacesyncproto.ObjectSyncMessage) (err error) {

	var fullRequest *spacesyncproto.ObjectSyncMessage
	res, err := s.treeCache.GetTree(ctx, s.spaceId, msg.TreeId)
	if err != nil {
		return
	}

	err = func() error {
		objTree := res.TreeContainer.Tree()
		objTree.Lock()
		defer res.Release()
		defer objTree.Unlock()

		if s.alreadyHaveHeads(objTree, update.Heads) {
			return nil
		}

		_, err = objTree.AddRawChanges(ctx, update.Changes...)
		if err != nil {
			return err
		}

		if s.alreadyHaveHeads(objTree, update.Heads) {
			return nil
		}

		fullRequest, err = s.factory.FullSyncRequest(objTree, update.Heads, update.SnapshotPath, msg.TrackingId)
		if err != nil {
			return err
		}
		return nil
	}()

	if fullRequest != nil {
		return s.syncClient.SendAsync(senderId, fullRequest)
	}
	return
}

func (s *syncHandler) handleFullSyncRequest(
	ctx context.Context,
	senderId string,
	request *spacesyncproto.ObjectFullSyncRequest,
	msg *spacesyncproto.ObjectSyncMessage) (err error) {
	var (
		fullResponse *spacesyncproto.ObjectSyncMessage
		header       = msg.RootChange
	)
	defer func() {
		if err != nil {
			s.syncClient.SendAsync(senderId, spacesyncproto.WrapError(err, header, msg.TreeId, msg.TrackingId))
		}
	}()

	res, err := s.treeCache.GetTree(ctx, s.spaceId, msg.TreeId)
	if err != nil {
		return
	}

	err = func() error {
		objTree := res.TreeContainer.Tree()
		objTree.Lock()
		defer res.Release()
		defer objTree.Unlock()

		if header == nil {
			header = objTree.Header()
		}

		if !s.alreadyHaveHeads(objTree, request.Heads) {
			_, err = objTree.AddRawChanges(ctx, request.Changes...)
			if err != nil {
				return err
			}
		}

		fullResponse, err = s.factory.FullSyncResponse(objTree, request.Heads, request.SnapshotPath, msg.TrackingId)
		return err
	}()

	if err != nil {
		return
	}
	return s.syncClient.SendAsync(senderId, fullResponse)
}

func (s *syncHandler) handleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *spacesyncproto.ObjectFullSyncResponse,
	msg *spacesyncproto.ObjectSyncMessage) (err error) {
	res, err := s.treeCache.GetTree(ctx, s.spaceId, msg.TreeId)
	if err != nil {
		return
	}

	err = func() error {
		objTree := res.TreeContainer.Tree()
		objTree.Lock()
		defer res.Release()
		defer objTree.Unlock()

		if s.alreadyHaveHeads(objTree, response.Heads) {
			return nil
		}

		_, err = objTree.AddRawChanges(ctx, response.Changes...)
		return err
	}()

	return
}

func (s *syncHandler) alreadyHaveHeads(t tree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(t.Heads(), heads) || t.HasChanges(heads...)
}
