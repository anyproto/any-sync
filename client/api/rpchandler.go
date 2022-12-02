package api

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
)

type rpcHandler struct {
	controller Controller
}

func (r *rpcHandler) LoadSpace(ctx context.Context, request *apiproto.LoadSpaceRequest) (resp *apiproto.LoadSpaceResponse, err error) {
	err = r.controller.LoadSpace(request.SpaceId)
	if err != nil {
		return
	}
	resp = &apiproto.LoadSpaceResponse{}
	return
}

func (r *rpcHandler) CreateSpace(ctx context.Context, request *apiproto.CreateSpaceRequest) (resp *apiproto.CreateSpaceResponse, err error) {
	id, err := r.controller.CreateSpace()
	if err != nil {
		return
	}
	resp = &apiproto.CreateSpaceResponse{Id: id}
	return
}

func (r *rpcHandler) DeriveSpace(ctx context.Context, request *apiproto.DeriveSpaceRequest) (resp *apiproto.DeriveSpaceResponse, err error) {
	id, err := r.controller.DeriveSpace()
	if err != nil {
		return
	}
	resp = &apiproto.DeriveSpaceResponse{Id: id}
	return
}

func (r *rpcHandler) CreateDocument(ctx context.Context, request *apiproto.CreateDocumentRequest) (resp *apiproto.CreateDocumentResponse, err error) {
	id, err := r.controller.CreateDocument(request.SpaceId)
	if err != nil {
		return
	}
	resp = &apiproto.CreateDocumentResponse{Id: id}
	return
}

func (r *rpcHandler) DeleteDocument(ctx context.Context, request *apiproto.DeleteDocumentRequest) (resp *apiproto.DeleteDocumentResponse, err error) {
	err = r.controller.DeleteDocument(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &apiproto.DeleteDocumentResponse{}
	return
}

func (r *rpcHandler) AddText(ctx context.Context, request *apiproto.AddTextRequest) (resp *apiproto.AddTextResponse, err error) {
	err = r.controller.AddText(request.SpaceId, request.DocumentId, request.Text)
	if err != nil {
		return
	}
	// TODO: update controller to add head
	resp = &apiproto.AddTextResponse{
		DocumentId: request.DocumentId,
	}
	return
}

func (r *rpcHandler) DumpTree(ctx context.Context, request *apiproto.DumpTreeRequest) (resp *apiproto.DumpTreeResponse, err error) {
	dump, err := r.controller.DumpDocumentTree(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &apiproto.DumpTreeResponse{
		Dump: dump,
	}
	return
}

func (r *rpcHandler) AllTrees(ctx context.Context, request *apiproto.AllTreesRequest) (resp *apiproto.AllTreesResponse, err error) {
	ids, err := r.controller.AllDocumentIds(request.SpaceId)
	if err != nil {
		return
	}
	// TODO: add getting heads to controller
	var trees []*apiproto.Tree
	for _, id := range ids {
		trees = append(trees, &apiproto.Tree{
			Id: id,
		})
	}
	resp = &apiproto.AllTreesResponse{Trees: trees}
	return
}

func (r *rpcHandler) AllSpaces(ctx context.Context, request *apiproto.AllSpacesRequest) (resp *apiproto.AllSpacesResponse, err error) {
	ids, err := r.controller.AllSpaceIds()
	if err != nil {
		return
	}
	resp = &apiproto.AllSpacesResponse{SpaceIds: ids}
	return
}
