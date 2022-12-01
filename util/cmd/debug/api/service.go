package api

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/client"
)

const CName = "debug.api"

var log = logger.NewNamed(CName)

type Service interface {
	CreateSpace(ctx context.Context, ip string, request *apiproto.CreateSpaceRequest) (resp *apiproto.CreateSpaceResponse, err error)
	app.Component
}

type service struct {
	client client.Service
}

func New() Service {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	s.client = a.MustComponent(client.CName).(client.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateSpace(ctx context.Context, ip string, request *apiproto.CreateSpaceRequest) (resp *apiproto.CreateSpaceResponse, err error) {
	cl, err := s.client.Get(ctx, ip)
	if err != nil {
		return
	}
	return cl.CreateSpace(ctx, request)
}
