package client

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/debug/clientdebugrpc/clientdebugrpcproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/drpcclient"
)

const CName = "debug.commands.client"

var log = logger.NewNamed(CName)

type Service interface {
	app.Component
	CreateSpace(ctx context.Context, ip string, request *clientdebugrpcproto.CreateSpaceRequest) (resp *clientdebugrpcproto.CreateSpaceResponse, err error)
	DeriveSpace(ctx context.Context, ip string, request *clientdebugrpcproto.DeriveSpaceRequest) (resp *clientdebugrpcproto.DeriveSpaceResponse, err error)
	CreateDocument(ctx context.Context, ip string, request *clientdebugrpcproto.CreateDocumentRequest) (resp *clientdebugrpcproto.CreateDocumentResponse, err error)
	DeleteDocument(ctx context.Context, ip string, request *clientdebugrpcproto.DeleteDocumentRequest) (resp *clientdebugrpcproto.DeleteDocumentResponse, err error)
	AddText(ctx context.Context, ip string, request *clientdebugrpcproto.AddTextRequest) (resp *clientdebugrpcproto.AddTextResponse, err error)
	DumpTree(ctx context.Context, ip string, request *clientdebugrpcproto.DumpTreeRequest) (resp *clientdebugrpcproto.DumpTreeResponse, err error)
	TreeParams(ctx context.Context, ip string, request *clientdebugrpcproto.TreeParamsRequest) (resp *clientdebugrpcproto.TreeParamsResponse, err error)
	AllTrees(ctx context.Context, ip string, request *clientdebugrpcproto.AllTreesRequest) (resp *clientdebugrpcproto.AllTreesResponse, err error)
	AllSpaces(ctx context.Context, ip string, request *clientdebugrpcproto.AllSpacesRequest) (resp *clientdebugrpcproto.AllSpacesResponse, err error)
	LoadSpace(ctx context.Context, ip string, request *clientdebugrpcproto.LoadSpaceRequest) (res *clientdebugrpcproto.LoadSpaceResponse, err error)
	Watch(ctx context.Context, ip string, request *clientdebugrpcproto.WatchRequest) (res *clientdebugrpcproto.WatchResponse, err error)
	Unwatch(ctx context.Context, ip string, request *clientdebugrpcproto.UnwatchRequest) (res *clientdebugrpcproto.UnwatchResponse, err error)
	PutFile(ctx context.Context, ip string, request *clientdebugrpcproto.PutFileRequest) (resp *clientdebugrpcproto.PutFileResponse, err error)
	GetFile(ctx context.Context, ip string, request *clientdebugrpcproto.GetFileRequest) (resp *clientdebugrpcproto.GetFileResponse, err error)
	DeleteFile(ctx context.Context, ip string, request *clientdebugrpcproto.DeleteFileRequest) (resp *clientdebugrpcproto.DeleteFileResponse, err error)
}

type service struct {
	client drpcclient.Service
}

func New() Service {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	s.client = a.MustComponent(drpcclient.CName).(drpcclient.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateSpace(ctx context.Context, ip string, request *clientdebugrpcproto.CreateSpaceRequest) (resp *clientdebugrpcproto.CreateSpaceResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.CreateSpace(ctx, request)
}

func (s *service) DeriveSpace(ctx context.Context, ip string, request *clientdebugrpcproto.DeriveSpaceRequest) (resp *clientdebugrpcproto.DeriveSpaceResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.DeriveSpace(ctx, request)
}

func (s *service) CreateDocument(ctx context.Context, ip string, request *clientdebugrpcproto.CreateDocumentRequest) (resp *clientdebugrpcproto.CreateDocumentResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.CreateDocument(ctx, request)
}

func (s *service) DeleteDocument(ctx context.Context, ip string, request *clientdebugrpcproto.DeleteDocumentRequest) (resp *clientdebugrpcproto.DeleteDocumentResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.DeleteDocument(ctx, request)
}

func (s *service) AddText(ctx context.Context, ip string, request *clientdebugrpcproto.AddTextRequest) (resp *clientdebugrpcproto.AddTextResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.AddText(ctx, request)
}

func (s *service) DumpTree(ctx context.Context, ip string, request *clientdebugrpcproto.DumpTreeRequest) (resp *clientdebugrpcproto.DumpTreeResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.DumpTree(ctx, request)
}

func (s *service) TreeParams(ctx context.Context, ip string, request *clientdebugrpcproto.TreeParamsRequest) (resp *clientdebugrpcproto.TreeParamsResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.TreeParams(ctx, request)
}

func (s *service) AllTrees(ctx context.Context, ip string, request *clientdebugrpcproto.AllTreesRequest) (resp *clientdebugrpcproto.AllTreesResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllTrees(ctx, request)
}

func (s *service) AllSpaces(ctx context.Context, ip string, request *clientdebugrpcproto.AllSpacesRequest) (resp *clientdebugrpcproto.AllSpacesResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllSpaces(ctx, request)
}

func (s *service) LoadSpace(ctx context.Context, ip string, request *clientdebugrpcproto.LoadSpaceRequest) (res *clientdebugrpcproto.LoadSpaceResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.LoadSpace(ctx, request)
}

func (s *service) Watch(ctx context.Context, ip string, request *clientdebugrpcproto.WatchRequest) (res *clientdebugrpcproto.WatchResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.Watch(ctx, request)
}

func (s *service) Unwatch(ctx context.Context, ip string, request *clientdebugrpcproto.UnwatchRequest) (res *clientdebugrpcproto.UnwatchResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.Unwatch(ctx, request)
}

func (s *service) PutFile(ctx context.Context, ip string, request *clientdebugrpcproto.PutFileRequest) (resp *clientdebugrpcproto.PutFileResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.PutFile(ctx, request)
}

func (s *service) GetFile(ctx context.Context, ip string, request *clientdebugrpcproto.GetFileRequest) (resp *clientdebugrpcproto.GetFileResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.GetFile(ctx, request)
}

func (s *service) DeleteFile(ctx context.Context, ip string, request *clientdebugrpcproto.DeleteFileRequest) (resp *clientdebugrpcproto.DeleteFileResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.DeleteFile(ctx, request)
}
