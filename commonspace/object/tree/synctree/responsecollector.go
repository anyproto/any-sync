package synctree

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

var ErrUnexpectedResponseType = errors.New("unexpected response type")

type responseCollector struct {
	heads   []string
	root    *treechangeproto.RawTreeChangeWithId
	changes []*treechangeproto.RawTreeChangeWithId
}

func newResponseCollector() *responseCollector {
	return &responseCollector{}
}

func (r *responseCollector) CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	treeResp, ok := resp.(Response)
	if !ok {
		return ErrUnexpectedResponseType
	}
	r.heads = treeResp.heads
	r.root = treeResp.root
	r.changes = append(r.changes, treeResp.changes...)
	return nil
}
