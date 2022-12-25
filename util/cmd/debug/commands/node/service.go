package node

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/debug/nodedebugrpc/nodedebugrpcproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/drpcclient"
)

const CName = "commands.node"

var log = logger.NewNamed(CName)

type Service interface {
	app.Component
	DumpTree(ctx context.Context, ip string, request *nodedebugrpcproto.DumpTreeRequest) (resp *nodedebugrpcproto.DumpTreeResponse, err error)
	TreeParams(ctx context.Context, ip string, request *nodedebugrpcproto.TreeParamsRequest) (resp *nodedebugrpcproto.TreeParamsResponse, err error)
	AllTrees(ctx context.Context, ip string, request *nodedebugrpcproto.AllTreesRequest) (resp *nodedebugrpcproto.AllTreesResponse, err error)
	AllSpaces(ctx context.Context, ip string, request *nodedebugrpcproto.AllSpacesRequest) (resp *nodedebugrpcproto.AllSpacesResponse, err error)
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

func (s *service) DumpTree(ctx context.Context, ip string, request *nodedebugrpcproto.DumpTreeRequest) (resp *nodedebugrpcproto.DumpTreeResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.DumpTree(ctx, request)
}

func (s *service) TreeParams(ctx context.Context, ip string, request *nodedebugrpcproto.TreeParamsRequest) (resp *nodedebugrpcproto.TreeParamsResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.TreeParams(ctx, request)
}

func (s *service) AllTrees(ctx context.Context, ip string, request *nodedebugrpcproto.AllTreesRequest) (resp *nodedebugrpcproto.AllTreesResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllTrees(ctx, request)
}

func (s *service) AllSpaces(ctx context.Context, ip string, request *nodedebugrpcproto.AllSpacesRequest) (resp *nodedebugrpcproto.AllSpacesResponse, err error) {
	cl, err := s.client.GetNode(ctx, ip)
	if err != nil {
		return
	}
	return cl.AllSpaces(ctx, request)
}
