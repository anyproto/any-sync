package space

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/space/spacesync"
)

type rpcServer struct {
	s *service
}

func (r rpcServer) HeadSync(ctx context.Context, request *spacesync.HeadSyncRequest) (*spacesync.HeadSyncResponse, error) {
	sp, err := r.s.get(ctx, request.SpaceId)
	if err != nil {
		return nil, err
	}
	return sp.HeadSync(ctx, request)
}
