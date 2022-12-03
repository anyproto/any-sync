package client

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/drpcclient"
)

const CName = "api.client"

var log = logger.NewNamed(CName)

type Service interface {
	app.Component
	CreateSpace(ctx context.Context, ip string, request *apiproto.CreateSpaceRequest) (resp *apiproto.CreateSpaceResponse, err error)
	DeriveSpace(ctx context.Context, ip string, request *apiproto.DeriveSpaceRequest) (resp *apiproto.DeriveSpaceResponse, err error)
	CreateDocument(ctx context.Context, ip string, request *apiproto.CreateDocumentRequest) (resp *apiproto.CreateDocumentResponse, err error)
	DeleteDocument(ctx context.Context, ip string, request *apiproto.DeleteDocumentRequest) (resp *apiproto.DeleteDocumentResponse, err error)
	AddText(ctx context.Context, ip string, request *apiproto.AddTextRequest) (resp *apiproto.AddTextResponse, err error)
	DumpTree(ctx context.Context, ip string, request *apiproto.DumpTreeRequest) (resp *apiproto.DumpTreeResponse, err error)
	TreeParams(ctx context.Context, ip string, request *apiproto.TreeParamsRequest) (resp *apiproto.TreeParamsResponse, err error)
	AllTrees(ctx context.Context, ip string, request *apiproto.AllTreesRequest) (resp *apiproto.AllTreesResponse, err error)
	AllSpaces(ctx context.Context, ip string, request *apiproto.AllSpacesRequest) (resp *apiproto.AllSpacesResponse, err error)
	LoadSpace(ctx context.Context, ip string, request *apiproto.LoadSpaceRequest) (res *apiproto.LoadSpaceResponse, err error)
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

func (s *service) CreateSpace(ctx context.Context, ip string, request *apiproto.CreateSpaceRequest) (resp *apiproto.CreateSpaceResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.CreateSpace(ctx, request)
}

func (s *service) DeriveSpace(ctx context.Context, ip string, request *apiproto.DeriveSpaceRequest) (resp *apiproto.DeriveSpaceResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.DeriveSpace(ctx, request)
}

func (s *service) CreateDocument(ctx context.Context, ip string, request *apiproto.CreateDocumentRequest) (resp *apiproto.CreateDocumentResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.CreateDocument(ctx, request)
}

func (s *service) DeleteDocument(ctx context.Context, ip string, request *apiproto.DeleteDocumentRequest) (resp *apiproto.DeleteDocumentResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.DeleteDocument(ctx, request)
}

func (s *service) AddText(ctx context.Context, ip string, request *apiproto.AddTextRequest) (resp *apiproto.AddTextResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.AddText(ctx, request)
}

func (s *service) DumpTree(ctx context.Context, ip string, request *apiproto.DumpTreeRequest) (resp *apiproto.DumpTreeResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.DumpTree(ctx, request)
}

func (s *service) TreeParams(ctx context.Context, ip string, request *apiproto.TreeParamsRequest) (resp *apiproto.TreeParamsResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.TreeParams(ctx, request)
}

func (s *service) AllTrees(ctx context.Context, ip string, request *apiproto.AllTreesRequest) (resp *apiproto.AllTreesResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllTrees(ctx, request)
}

func (s *service) AllSpaces(ctx context.Context, ip string, request *apiproto.AllSpacesRequest) (resp *apiproto.AllSpacesResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllSpaces(ctx, request)
}

func (s *service) LoadSpace(ctx context.Context, ip string, request *apiproto.LoadSpaceRequest) (res *apiproto.LoadSpaceResponse, err error) {
	cl, err := s.client.GetClient(ctx, ip)
	if err != nil {
		return
	}
	return cl.LoadSpace(ctx, request)
}
