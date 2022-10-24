package nodespace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
)

type rpcHandler struct {
	s *service
}

func (r *rpcHandler) PushSpace(ctx context.Context, req *spacesyncproto.PushSpaceRequest) (resp *spacesyncproto.PushSpaceResponse, err error) {
	_, err = r.s.GetSpace(ctx, req.SpaceHeader.Id)
	if err == nil {
		err = spacesyncproto.ErrSpaceExists
		return
	}
	if err != storage.ErrSpaceStorageMissing {
		err = spacesyncproto.ErrUnexpected
		return
	}

	payload := storage.SpaceStorageCreatePayload{
		RecWithId: &aclrecordproto.RawACLRecordWithId{
			Payload: req.AclPayload,
			Id:      req.AclPayloadId,
		},
		SpaceHeaderWithId: req.SpaceHeader,
	}
	st, err := r.s.spaceStorageProvider.CreateSpaceStorage(payload)
	if err != nil {
		err = spacesyncproto.ErrUnexpected
		if err == storage.ErrSpaceStorageExists {
			err = spacesyncproto.ErrSpaceExists
		}
		return
	}
	st.Close()
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
