package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
)

type RpcHandler interface {
	HeadSync(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error)
	Stream(stream spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream) error
}

type rpcHandler struct {
	s *space
}

func (r *rpcHandler) HeadSync(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	return r.s.HeadSync().HandleRangeRequest(ctx, req)
}

func (r *rpcHandler) Stream(stream spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream) (err error) {
	// TODO: if needed we can launch full sync here
	return r.s.ObjectSync().StreamPool().AddAndReadStreamSync(stream)
}
