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
	CreateSpace(ctx context.Context, ip string, request *apiproto.CreateSpaceRequest) (resp *apiproto.CreateSpaceResponse, err error)
	app.Component
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
