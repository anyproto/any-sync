package clientdebugrpc

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/debug/clientdebugrpc/clientdebugrpcproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncstatus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"github.com/ipfs/go-cid"
	"go.uber.org/zap"
	"io"
	"math/rand"
	"os"
)

type rpcHandler struct {
	spaceService   clientspace.Service
	storageService storage.ClientStorage
	docService     document.Service
	account        accountservice.Service
	file           fileservice.FileService
}

func (r *rpcHandler) Watch(ctx context.Context, request *clientdebugrpcproto.WatchRequest) (resp *clientdebugrpcproto.WatchResponse, err error) {
	space, err := r.spaceService.GetSpace(context.Background(), request.SpaceId)
	if err != nil {
		return
	}
	watcher := space.SyncStatus().(syncstatus.SyncStatusWatcher)
	watcher.Watch(request.TreeId)
	resp = &clientdebugrpcproto.WatchResponse{}
	return
}

func (r *rpcHandler) Unwatch(ctx context.Context, request *clientdebugrpcproto.UnwatchRequest) (resp *clientdebugrpcproto.UnwatchResponse, err error) {
	space, err := r.spaceService.GetSpace(context.Background(), request.SpaceId)
	if err != nil {
		return
	}
	watcher := space.SyncStatus().(syncstatus.SyncStatusWatcher)
	watcher.Unwatch(request.TreeId)
	resp = &clientdebugrpcproto.UnwatchResponse{}
	return
}

func (r *rpcHandler) LoadSpace(ctx context.Context, request *clientdebugrpcproto.LoadSpaceRequest) (resp *clientdebugrpcproto.LoadSpaceResponse, err error) {
	_, err = r.spaceService.GetSpace(context.Background(), request.SpaceId)
	if err != nil {
		return
	}
	resp = &clientdebugrpcproto.LoadSpaceResponse{}
	return
}

func (r *rpcHandler) CreateSpace(ctx context.Context, request *clientdebugrpcproto.CreateSpaceRequest) (resp *clientdebugrpcproto.CreateSpaceResponse, err error) {
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
	resp = &clientdebugrpcproto.CreateSpaceResponse{Id: id}
	return
}

func (r *rpcHandler) DeriveSpace(ctx context.Context, request *clientdebugrpcproto.DeriveSpaceRequest) (resp *clientdebugrpcproto.DeriveSpaceResponse, err error) {
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
	resp = &clientdebugrpcproto.DeriveSpaceResponse{Id: id}
	return
}

func (r *rpcHandler) CreateDocument(ctx context.Context, request *clientdebugrpcproto.CreateDocumentRequest) (resp *clientdebugrpcproto.CreateDocumentResponse, err error) {
	id, err := r.docService.CreateDocument(request.SpaceId)
	if err != nil {
		return
	}
	resp = &clientdebugrpcproto.CreateDocumentResponse{Id: id}
	return
}

func (r *rpcHandler) DeleteDocument(ctx context.Context, request *clientdebugrpcproto.DeleteDocumentRequest) (resp *clientdebugrpcproto.DeleteDocumentResponse, err error) {
	err = r.docService.DeleteDocument(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &clientdebugrpcproto.DeleteDocumentResponse{}
	return
}

func (r *rpcHandler) AddText(ctx context.Context, request *clientdebugrpcproto.AddTextRequest) (resp *clientdebugrpcproto.AddTextResponse, err error) {
	root, head, err := r.docService.AddText(request.SpaceId, request.DocumentId, request.Text, request.IsSnapshot)
	if err != nil {
		return
	}
	resp = &clientdebugrpcproto.AddTextResponse{
		DocumentId: request.DocumentId,
		HeadId:     head,
		RootId:     root,
	}
	return
}

func (r *rpcHandler) DumpTree(ctx context.Context, request *clientdebugrpcproto.DumpTreeRequest) (resp *clientdebugrpcproto.DumpTreeResponse, err error) {
	dump, err := r.docService.DumpDocumentTree(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &clientdebugrpcproto.DumpTreeResponse{
		Dump: dump,
	}
	return
}

func (r *rpcHandler) AllTrees(ctx context.Context, request *clientdebugrpcproto.AllTreesRequest) (resp *clientdebugrpcproto.AllTreesResponse, err error) {
	heads, err := r.docService.AllDocumentHeads(request.SpaceId)
	if err != nil {
		return
	}
	var trees []*clientdebugrpcproto.Tree
	for _, head := range heads {
		trees = append(trees, &clientdebugrpcproto.Tree{
			Id:    head.Id,
			Heads: head.Heads,
		})
	}
	resp = &clientdebugrpcproto.AllTreesResponse{Trees: trees}
	return
}

func (r *rpcHandler) AllSpaces(ctx context.Context, request *clientdebugrpcproto.AllSpacesRequest) (resp *clientdebugrpcproto.AllSpacesResponse, err error) {
	ids, err := r.storageService.AllSpaceIds()
	if err != nil {
		return
	}
	resp = &clientdebugrpcproto.AllSpacesResponse{SpaceIds: ids}
	return
}

func (r *rpcHandler) TreeParams(ctx context.Context, request *clientdebugrpcproto.TreeParamsRequest) (resp *clientdebugrpcproto.TreeParamsResponse, err error) {
	root, heads, err := r.docService.TreeParams(request.SpaceId, request.DocumentId)
	if err != nil {
		return
	}
	resp = &clientdebugrpcproto.TreeParamsResponse{
		RootId:  root,
		HeadIds: heads,
	}
	return
}

func (r *rpcHandler) PutFile(ctx context.Context, request *clientdebugrpcproto.PutFileRequest) (*clientdebugrpcproto.PutFileResponse, error) {
	f, err := os.Open(request.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n, err := r.file.AddFile(ctx, f)
	if err != nil {
		return nil, err
	}
	return &clientdebugrpcproto.PutFileResponse{
		Hash: n.Cid().String(),
	}, nil
}

func (r *rpcHandler) GetFile(ctx context.Context, request *clientdebugrpcproto.GetFileRequest) (*clientdebugrpcproto.GetFileResponse, error) {
	c, err := cid.Parse(request.Hash)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(request.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rd, err := r.file.GetFile(ctx, c)
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	wr, err := io.Copy(f, rd)
	if err != nil && err != io.EOF {
		return nil, err
	}
	log.Info("copied bytes", zap.Int64("size", wr))
	return &clientdebugrpcproto.GetFileResponse{
		Path: request.Path,
	}, nil
}

func (r *rpcHandler) DeleteFile(ctx context.Context, request *clientdebugrpcproto.DeleteFileRequest) (*clientdebugrpcproto.DeleteFileResponse, error) {
	//TODO implement me
	panic("implement me")
}
