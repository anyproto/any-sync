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
		c, err := cid.Cast(req.Cid)
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
	c, err := cid.Cast(req.Cid)
	if err != nil {
		return nil, err
	}
	b, err := blocks.NewBlockWithCid(req.Data, c)
	if err != nil {
		return nil, err
	}
	if err = r.store.Add(fileblockstore.CtxWithSpaceId(ctx, req.SpaceId), []blocks.Block{b}); err != nil {
		return nil, fileprotoerr.ErrUnexpected
	}
	return &fileproto.PushBlockResponse{}, nil
}

func (r *rpcHandler) DeleteBlocks(ctx context.Context, req *fileproto.DeleteBlocksRequest) (*fileproto.DeleteBlocksResponse, error) {
	for _, cd := range req.Cid {
		c, err := cid.Cast(cd)
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
