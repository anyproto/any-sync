package nodespace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
)

type rpcHandler struct {
	s *service
}

func (r *rpcHandler) HeadSync(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	sp, err := r.s.GetSpace(ctx, req.SpaceId)
	if err != nil {
		return nil, err
	}
	return sp.SpaceSyncRpc().HeadSync(ctx, req)
}

func (r *rpcHandler) Stream(stream spacesyncproto.DRPCSpace_StreamStream) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}
	sp, err := r.s.GetSpace(stream.Context(), msg.SpaceId)
	if err != nil {
		return err
	}
	return sp.SpaceSyncRpc().Stream(stream)
}
