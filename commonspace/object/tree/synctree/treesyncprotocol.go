package synctree

import (
	"context"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/slice"
	"go.uber.org/zap"
)

type TreeSyncProtocol interface {
	HeadUpdate(ctx context.Context, senderId string, update *treechangeproto.TreeHeadUpdate) (request *treechangeproto.TreeSyncMessage, err error)
	FullSyncRequest(ctx context.Context, senderId string, request *treechangeproto.TreeFullSyncRequest) (response *treechangeproto.TreeSyncMessage, err error)
	FullSyncResponse(ctx context.Context, senderId string, response *treechangeproto.TreeFullSyncResponse) (err error)
}

type treeSyncProtocol struct {
	log        logger.CtxLogger
	spaceId    string
	objTree    objecttree.ObjectTree
	reqFactory RequestFactory
}

func newTreeSyncProtocol(spaceId string, objTree objecttree.ObjectTree, reqFactory RequestFactory) *treeSyncProtocol {
	return &treeSyncProtocol{
		log:        log.With(zap.String("spaceId", spaceId), zap.String("treeId", objTree.Id())),
		spaceId:    spaceId,
		objTree:    objTree,
		reqFactory: reqFactory,
	}
}

func (t *treeSyncProtocol) HeadUpdate(ctx context.Context, senderId string, update *treechangeproto.TreeHeadUpdate) (fullRequest *treechangeproto.TreeSyncMessage, err error) {
	var (
		isEmptyUpdate = len(update.Changes) == 0
		objTree       = t.objTree
	)
	log := t.log.With(
		zap.String("senderId", senderId),
		zap.Strings("update heads", update.Heads),
		zap.Int("len(update changes)", len(update.Changes)))
	log.DebugCtx(ctx, "received head update message")

	defer func() {
		if err != nil {
			log.ErrorCtx(ctx, "head update finished with error", zap.Error(err))
		} else if fullRequest != nil {
			cnt := fullRequest.Content.GetFullSyncRequest()
			log = log.With(zap.Strings("request heads", cnt.Heads), zap.Int("len(request changes)", len(cnt.Changes)))
			log.DebugCtx(ctx, "returning full sync request")
		} else {
			if !isEmptyUpdate {
				log.DebugCtx(ctx, "head update finished correctly")
			}
		}
	}()

	// isEmptyUpdate is sent when the tree is brought up from cache
	if isEmptyUpdate {
		headEquals := slice.UnsortedEquals(objTree.Heads(), update.Heads)
		log.DebugCtx(ctx, "is empty update", zap.Bool("headEquals", headEquals))
		if headEquals {
			return
		}

		// we need to sync in any case
		fullRequest, err = t.reqFactory.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)
		return
	}

	if t.hasHeads(objTree, update.Heads) {
		return
	}

	_, err = objTree.AddRawChanges(ctx, objecttree.RawChangesPayload{
		NewHeads:   update.Heads,
		RawChanges: update.Changes,
	})
	if err != nil {
		return
	}

	if t.hasHeads(objTree, update.Heads) {
		return
	}

	fullRequest, err = t.reqFactory.CreateFullSyncRequest(objTree, update.Heads, update.SnapshotPath)
	return
}

func (t *treeSyncProtocol) FullSyncRequest(ctx context.Context, senderId string, request *treechangeproto.TreeFullSyncRequest) (fullResponse *treechangeproto.TreeSyncMessage, err error) {
	var (
		objTree = t.objTree
	)
	log := t.log.With(zap.String("senderId", senderId),
		zap.Strings("request heads", request.Heads),
		zap.Int("len(request changes)", len(request.Changes)))
	log.DebugCtx(ctx, "received full sync request message")

	defer func() {
		if err != nil {
			log.ErrorCtx(ctx, "full sync request finished with error", zap.Error(err))
		} else if fullResponse != nil {
			cnt := fullResponse.Content.GetFullSyncResponse()
			log = log.With(zap.Strings("response heads", cnt.Heads), zap.Int("len(response changes)", len(cnt.Changes)))
			log.DebugCtx(ctx, "full sync response sent")
		}
	}()

	if len(request.Changes) != 0 && !t.hasHeads(objTree, request.Heads) {
		_, err = objTree.AddRawChanges(ctx, objecttree.RawChangesPayload{
			NewHeads:   request.Heads,
			RawChanges: request.Changes,
		})
		if err != nil {
			return
		}
	}
	fullResponse, err = t.reqFactory.CreateFullSyncResponse(objTree, request.Heads, request.SnapshotPath)
	return
}

func (t *treeSyncProtocol) FullSyncResponse(ctx context.Context, senderId string, response *treechangeproto.TreeFullSyncResponse) (err error) {
	var (
		objTree = t.objTree
	)
	log := log.With(
		zap.Strings("heads", response.Heads),
		zap.Int("len(changes)", len(response.Changes)))
	log.DebugCtx(ctx, "received full sync response message")
	defer func() {
		if err != nil {
			log.ErrorCtx(ctx, "full sync response failed", zap.Error(err))
		} else {
			log.DebugCtx(ctx, "full sync response succeeded")
		}
	}()
	if t.hasHeads(objTree, response.Heads) {
		return
	}

	_, err = objTree.AddRawChanges(ctx, objecttree.RawChangesPayload{
		NewHeads:   response.Heads,
		RawChanges: response.Changes,
	})
	return
}

func (t *treeSyncProtocol) hasHeads(ot objecttree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(ot.Heads(), heads) || ot.HasChanges(heads...)
}
