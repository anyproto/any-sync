package api

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/secure"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/nodespace"
	nodestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/node/storage"
	"storj.io/drpc"
)

const CName = "api.service"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{BaseDrpcServer: server.NewBaseDrpcServer()}
}

type Service interface {
	app.ComponentRunnable
	drpc.Mux
}

type service struct {
	transport      secure.Service
	cfg            *config.Config
	treeCache      treegetter.TreeGetter
	spaceService   nodespace.Service
	storageService nodestorage.NodeStorage
	*server.BaseDrpcServer
}

func (s *service) Init(a *app.App) (err error) {
	s.treeCache = a.MustComponent(treegetter.CName).(treegetter.TreeGetter)
	s.spaceService = a.MustComponent(nodespace.CName).(nodespace.Service)
	s.storageService = a.MustComponent(storage.CName).(nodestorage.NodeStorage)
	s.cfg = a.MustComponent(config.CName).(*config.Config)
	s.transport = a.MustComponent(secure.CName).(secure.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	params := server.Params{
		BufferSizeMb:  s.cfg.Stream.MaxMsgSizeMb,
		TimeoutMillis: s.cfg.Stream.TimeoutMilliseconds,
		ListenAddrs:   s.cfg.APIServer.ListenAddrs,
		Wrapper: func(handler drpc.Handler) drpc.Handler {
			return handler
		},
		Converter: s.transport.BasicListener,
	}
	err = s.BaseDrpcServer.Run(ctx, params)
	if err != nil {
		return
	}
	return apiproto.DRPCRegisterNodeApi(s, &rpcHandler{s.treeCache, s.spaceService, s.storageService})
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.BaseDrpcServer.Close(ctx)
}
