package commonspace

import (
	"context"
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
	return r.s.DiffService().HandleRangeRequest(ctx, req)
}

func (r *rpcHandler) Stream(stream spacesyncproto.DRPCSpace_StreamStream) (err error) {
	return r.s.SyncService().SyncClient().AddAndReadStreamSync(stream)
}
