package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

type responseCollector struct {
	handler syncdeps.ObjectSyncHandler
}

func (r *responseCollector) NewResponse() syncdeps.Response {
	return &Response{}
}

func newResponseCollector(handler syncdeps.ObjectSyncHandler) *responseCollector {
	return &responseCollector{handler: handler}
}

func (r *responseCollector) CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	return r.handler.HandleResponse(ctx, peerId, objectId, resp)
}
