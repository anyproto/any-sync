package node

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/drpcclient"
)

const CName = "api.node"

var log = logger.NewNamed(CName)

type Service interface {
	app.Component
	DumpTree(ctx context.Context, ip string, request *apiproto.DumpTreeRequest) (resp *apiproto.DumpTreeResponse, err error)
	TreeParams(ctx context.Context, ip string, request *apiproto.TreeParamsRequest) (resp *apiproto.TreeParamsResponse, err error)
	AllTrees(ctx context.Context, ip string, request *apiproto.AllTreesRequest) (resp *apiproto.AllTreesResponse, err error)
	AllSpaces(ctx context.Context, ip string, request *apiproto.AllSpacesRequest) (resp *apiproto.AllSpacesResponse, err error)
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

func (s *service) DumpTree(ctx context.Context, ip string, request *apiproto.DumpTreeRequest) (resp *apiproto.DumpTreeResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.DumpTree(ctx, request)
}

func (s *service) TreeParams(ctx context.Context, ip string, request *apiproto.TreeParamsRequest) (resp *apiproto.TreeParamsResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.TreeParams(ctx, request)
}

func (s *service) AllTrees(ctx context.Context, ip string, request *apiproto.AllTreesRequest) (resp *apiproto.AllTreesResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllTrees(ctx, request)
}

func (s *service) AllSpaces(ctx context.Context, ip string, request *apiproto.AllSpacesRequest) (resp *apiproto.AllSpacesResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllSpaces(ctx, request)
}
