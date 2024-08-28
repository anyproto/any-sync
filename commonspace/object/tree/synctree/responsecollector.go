package synctree

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type fullResponseCollector struct {
	heads   []string
	root    *treechangeproto.RawTreeChangeWithId
	changes []*treechangeproto.RawTreeChangeWithId
}

func newFullResponseCollector() *fullResponseCollector {
	return &fullResponseCollector{}
}

func (r *fullResponseCollector) CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	treeResp, ok := resp.(*response.Response)
	if !ok {
		return ErrUnexpectedResponseType
	}
	r.heads = treeResp.Heads
	r.root = treeResp.Root
	r.changes = append(r.changes, treeResp.Changes...)
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
