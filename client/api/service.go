package api

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document"
	clientstorage "github.com/anytypeio/go-anytype-infrastructure-experiments/client/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/secure"
	"net/http"
	"storj.io/drpc"
)

const CName = "api.service"

var log = logger.NewNamed("api")

func New() Service {
	return &service{BaseDrpcServer: server.NewBaseDrpcServer()}
}

type Service interface {
	app.ComponentRunnable
	drpc.Mux
}

type service struct {
	controller Controller
	srv        *http.Server
	cfg        *config.Config
	*server.BaseDrpcServer
}

func (s *service) Init(a *app.App) (err error) {
	s.controller = newController(
		a.MustComponent(clientspace.CName).(clientspace.Service),
		a.MustComponent(storage.CName).(clientstorage.ClientStorage),
		a.MustComponent(document.CName).(document.Service),
		a.MustComponent(account.CName).(account.Service))
	s.cfg = a.MustComponent(config.CName).(*config.Config)
	s.BaseDrpcServer.Init(a.MustComponent(secure.CName).(secure.Service))
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	err = s.BaseDrpcServer.Run(ctx, s.cfg.APIServer.ListenAddrs, func(handler drpc.Handler) drpc.Handler {
		return handler
	})
	if err != nil {
		return
	}
	return apiproto.DRPCRegisterClientApi(s, &rpcHandler{s.controller})
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.BaseDrpcServer.Close(ctx)
}
