package api

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"math/rand"
	"sync"
)

type rpcHandler struct {
	spaceService   clientspace.Service
	storageService storage.ClientStorage
	docService     document.Service
	account        account.Service
	treeWatcher    *watcher
	sync.Mutex
}

func (r *rpcHandler) Watch(ctx context.Context, request *apiproto.WatchRequest) (resp *apiproto.WatchResponse, err error) {
	space, err := r.spaceService.GetSpace(context.Background(), request.SpaceId)
	if err != nil {
		return
	}
	r.Lock()
	defer r.Unlock()

	ch := make(chan bool)
	r.treeWatcher = newWatcher(request.SpaceId, request.TreeId, ch)
	space.StatusService().Watch(request.TreeId, ch)
	go r.treeWatcher.run()
	resp = &apiproto.WatchResponse{}
	return
}

func (r *rpcHandler) Unwatch(ctx context.Context, request *apiproto.UnwatchRequest) (resp *apiproto.UnwatchResponse, err error) {
	space, err := r.spaceService.GetSpace(context.Background(), request.SpaceId)
	if err != nil {
		return
	}
	var treeWatcher *watcher
	space.StatusService().Unwatch(request.TreeId)

	r.Lock()
	if r.treeWatcher != nil {
		treeWatcher = r.treeWatcher
	}
	r.Unlock()

	treeWatcher.close()

	r.Lock()
	if r.treeWatcher == treeWatcher {
		r.treeWatcher = nil
	}
	r.Unlock()
	resp = &apiproto.UnwatchResponse{}
	return
}

func (r *rpcHandler) LoadSpace(ctx context.Context, request *apiproto.LoadSpaceRequest) (resp *apiproto.LoadSpaceResponse, err error) {
	_, err = r.spaceService.GetSpace(context.Background(), request.SpaceId)
	if err != nil {
		return
	}
	resp = &apiproto.LoadSpaceResponse{}
	return
}

func (r *rpcHandler) CreateSpace(ctx context.Context, request *apiproto.CreateSpaceRequest) (resp *apiproto.CreateSpaceResponse, err error) {
	key, err := symmetric.NewRandom()
	if err != nil {
		return
	}
	sp, err := r.spaceService.CreateSpace(context.Background(), commonspace.SpaceCreatePayload{
		SigningKey:     r.account.Account().SignKey,
		EncryptionKey:  r.account.Account().EncKey,
		ReadKey:        key.Bytes(),
		ReplicationKey: rand.Uint64(),
	})
	if err != nil {
		return
	}
	id := sp.Id()
	if err != nil {
		return
	}
	resp = &apiproto.CreateSpaceResponse{Id: id}
	return
}

func (r *rpcHandler) DeriveSpace(ctx context.Context, request *apiproto.DeriveSpaceRequest) (resp *apiproto.DeriveSpaceResponse, err error) {
	sp, err := r.spaceService.DeriveSpace(context.Background(), commonspace.SpaceDerivePayload{
		SigningKey:    r.account.Account().SignKey,
		EncryptionKey: r.account.Account().EncKey,
	})
	if err != nil {
		return
	}
	id := sp.Id()
	if err != nil {
		return
	}
	resp = &apiproto.DeriveSpaceResponse{Id: id}
	return
}

func (r *rpcHandler) CreateDocument(ctx context.Context, request *apiproto.CreateDocumentRequest) (resp *apiproto.CreateDocumentResponse, err error) {
	id, err := r.docService.CreateDocument(request.SpaceId)
	if err != nil {
		return
	}
	resp = &apiproto.CreateDocumentResponse{Id: id}
	return
}

func (r *rpcHandler) DeleteDocument(ctx context.Context, request *apiproto.DeleteDocumentRequest) (resp *apiproto.DeleteDocumentResponse, err error) {
	err = r.docService.DeleteDocument(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &apiproto.DeleteDocumentResponse{}
	return
}

func (r *rpcHandler) AddText(ctx context.Context, request *apiproto.AddTextRequest) (resp *apiproto.AddTextResponse, err error) {
	root, head, err := r.docService.AddText(request.SpaceId, request.DocumentId, request.Text, request.IsSnapshot)
	if err != nil {
		return
	}
	resp = &apiproto.AddTextResponse{
		DocumentId: request.DocumentId,
		HeadId:     head,
		RootId:     root,
	}
	return
}

func (r *rpcHandler) DumpTree(ctx context.Context, request *apiproto.DumpTreeRequest) (resp *apiproto.DumpTreeResponse, err error) {
	dump, err := r.docService.DumpDocumentTree(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &apiproto.DumpTreeResponse{
		Dump: dump,
	}
	return
}

func (r *rpcHandler) AllTrees(ctx context.Context, request *apiproto.AllTreesRequest) (resp *apiproto.AllTreesResponse, err error) {
	heads, err := r.docService.AllDocumentHeads(request.SpaceId)
	if err != nil {
		return
	}
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

func (r *rpcHandler) TreeParams(ctx context.Context, request *apiproto.TreeParamsRequest) (resp *apiproto.TreeParamsResponse, err error) {
	root, heads, err := r.docService.TreeParams(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &apiproto.TreeParamsResponse{
		RootId:  root,
		HeadIds: heads,
	}
	return
}
