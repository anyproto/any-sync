package synctree

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type fullResponseCollector struct {
	deps       BuildDeps
	heads      []string
	root       *treechangeproto.RawTreeChangeWithId
	changes    []*treechangeproto.RawTreeChangeWithId
	objectTree objecttree.ObjectTree
}

func newFullResponseCollector(deps BuildDeps) *fullResponseCollector {
	return &fullResponseCollector{
		deps: deps,
	}
}

func (r *fullResponseCollector) CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	treeResp, ok := resp.(*response.Response)
	if !ok {
		return ErrUnexpectedResponseType
	}
	if r.objectTree == nil {
		createPayload := treestorage.TreeStorageCreatePayload{
			RootRawChange: treeResp.Root,
			Changes:       treeResp.Changes,
			Heads:         treeResp.Heads,
		}
		validator := r.deps.ValidateObjectTree
		if validator == nil {
			validator = objecttree.ValidateRawTreeDefault
		}
		objTree, err := validator(createPayload, r.deps.SpaceStorage, r.deps.AclList)
		if err != nil {
			return err
		}
		r.objectTree = objTree
		return nil
	}
	_, err := r.objectTree.AddRawChanges(ctx, objecttree.RawChangesPayload{
		NewHeads:   treeResp.Heads,
		RawChanges: treeResp.Changes,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *fullResponseCollector) NewResponse() syncdeps.Response {
	return &response.Response{}
}

type responseCollector struct {
	handler syncdeps.ObjectSyncHandler
}

func newResponseCollector(handler syncdeps.ObjectSyncHandler) *responseCollector {
	return &responseCollector{handler: handler}
}

func (r *responseCollector) CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	return r.handler.HandleResponse(ctx, peerId, objectId, resp)
}

func (r *responseCollector) NewResponse() syncdeps.Response {
	return &response.Response{}
}
