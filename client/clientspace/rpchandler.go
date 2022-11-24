package clientspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
)

type rpcHandler struct {
	s *service
}

func (r *rpcHandler) PullSpace(ctx context.Context, request *spacesyncproto.PullSpaceRequest) (resp *spacesyncproto.PullSpaceResponse, err error) {
	sp, err := r.s.GetSpace(ctx, request.Id)
	if err != nil {
		if err != spacesyncproto.ErrSpaceMissing {
			err = spacesyncproto.ErrUnexpected
		}
		return
	}

	spaceDesc, err := sp.Description()
	if err != nil {
		err = spacesyncproto.ErrUnexpected
		return
	}

	resp = &spacesyncproto.PullSpaceResponse{
		Payload: &spacesyncproto.SpacePayload{
			SpaceHeader:            spaceDesc.SpaceHeader,
			AclPayloadId:           spaceDesc.AclId,
			AclPayload:             spaceDesc.AclPayload,
			SpaceSettingsPayload:   spaceDesc.SpaceSettingsPayload,
			SpaceSettingsPayloadId: spaceDesc.SpaceSettingsId,
		},
	}
	return
}

func (r *rpcHandler) PushSpace(ctx context.Context, req *spacesyncproto.PushSpaceRequest) (resp *spacesyncproto.PushSpaceResponse, err error) {
	description := commonspace.SpaceDescription{
		SpaceHeader:          req.Payload.SpaceHeader,
		AclId:                req.Payload.AclPayloadId,
		AclPayload:           req.Payload.AclPayload,
		SpaceSettingsPayload: req.Payload.SpaceSettingsPayload,
		SpaceSettingsId:      req.Payload.SpaceSettingsPayloadId,
	}
	ctx = context.WithValue(ctx, commonspace.AddSpaceCtxKey, description)
	_, err = r.s.GetSpace(ctx, description.SpaceHeader.GetId())
	if err != nil {
		return
	}
	resp = &spacesyncproto.PushSpaceResponse{}
	return
}

func (r *rpcHandler) HeadSync(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	sp, err := r.s.GetSpace(ctx, req.SpaceId)
	if err != nil {
		return nil, spacesyncproto.ErrSpaceMissing
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
		return spacesyncproto.ErrSpaceMissing
	}
	return sp.SpaceSyncRpc().Stream(stream)
}
