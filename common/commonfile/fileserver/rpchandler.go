package fileserver

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto/fileprotoerr"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

type rpcHandler struct {
	store fileblockstore.BlockStore
}

func (r *rpcHandler) GetBlocks(stream fileproto.DRPCFile_GetBlocksStream) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		resp := &fileproto.GetBlockResponse{
			Cid: req.Cid,
		}
		_, c, err := cid.CidFromBytes(req.Cid)
		if err != nil {
			resp.Code = fileproto.CIDError_CIDErrorUnexpected
		} else {
			b, err := r.store.Get(fileblockstore.CtxWithSpaceId(stream.Context(), req.SpaceId), c)
			if err != nil {
				if err == fileblockstore.ErrCIDNotFound {
					resp.Code = fileproto.CIDError_CIDErrorNotFound
				} else {
					resp.Code = fileproto.CIDError_CIDErrorUnexpected
				}
			} else {
				resp.Data = b.RawData()
			}
		}
		if err = stream.Send(resp); err != nil {
			return err
		}
	}
}

func (r *rpcHandler) PushBlock(ctx context.Context, req *fileproto.PushBlockRequest) (*fileproto.PushBlockResponse, error) {
	if err := r.store.Add(fileblockstore.CtxWithSpaceId(ctx, req.SpaceId), []blocks.Block{
		blocks.NewBlock(req.Data),
	}); err != nil {
		return nil, fileprotoerr.ErrUnexpected
	}
	return &fileproto.PushBlockResponse{}, nil
}

func (r *rpcHandler) DeleteBlocks(ctx context.Context, req *fileproto.DeleteBlocksRequest) (*fileproto.DeleteBlocksResponse, error) {
	for _, cd := range req.Cid {
		_, c, err := cid.CidFromBytes(cd)
		if err == nil {
			if err = r.store.Delete(fileblockstore.CtxWithSpaceId(ctx, req.SpaceId), c); err != nil {
				// TODO: log
			}
		}
	}
	return &fileproto.DeleteBlocksResponse{}, nil
}

func (r *rpcHandler) Check(ctx context.Context, request *fileproto.CheckRequest) (*fileproto.CheckResponse, error) {
	resp := &fileproto.CheckResponse{}
	if withSpaceIds, ok := r.store.(fileblockstore.BlockStoreSpaceIds); ok {
		resp.SpaceIds = withSpaceIds.SpaceIds()
	}
	return resp, nil
}
