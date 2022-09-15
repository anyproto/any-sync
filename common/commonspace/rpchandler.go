package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
)

type RpcHandler interface {
	HeadSync(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error)
	Stream(stream spacesyncproto.DRPCSpace_StreamStream) error
}

type rpcHandler struct {
	s *space
}

func (r *rpcHandler) HeadSync(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	return remotediff.HandlerRangeRequest(ctx, r.s.diff, req)
}

func (r *rpcHandler) Stream(stream spacesyncproto.DRPCSpace_StreamStream) error {
	return r.s.SyncService().StreamPool().AddStream(stream)
}
