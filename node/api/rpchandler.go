package api

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/nodespace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/storage"
)

type rpcHandler struct {
	treeCache      treegetter.TreeGetter
	spaceService   nodespace.Service
	storageService storage.NodeStorage
}

func (r *rpcHandler) DumpTree(ctx context.Context, request *apiproto.DumpTreeRequest) (resp *apiproto.DumpTreeResponse, err error) {
	tree, err := r.treeCache.GetTree(context.Background(), request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	dump, err := tree.DebugDump()
	if err != nil {
		return
	}
	resp = &apiproto.DumpTreeResponse{
		Dump: dump,
	}
	return
}

func (r *rpcHandler) AllTrees(ctx context.Context, request *apiproto.AllTreesRequest) (resp *apiproto.AllTreesResponse, err error) {
	space, err := r.spaceService.GetSpace(ctx, request.SpaceId)
	if err != nil {
		return
	}
	heads := space.DebugAllHeads()
	var trees []*apiproto.Tree
	for _, head := range heads {
		trees = append(trees, &apiproto.Tree{
			Id:    head.Id,
			Heads: head.Heads,
		})
	}
	resp = &apiproto.AllTreesResponse{Trees: trees}
	return
}

func (r *rpcHandler) AllSpaces(ctx context.Context, request *apiproto.AllSpacesRequest) (resp *apiproto.AllSpacesResponse, err error) {
	ids, err := r.storageService.AllSpaceIds()
	if err != nil {
		return
	}
	resp = &apiproto.AllSpacesResponse{SpaceIds: ids}
	return
}
