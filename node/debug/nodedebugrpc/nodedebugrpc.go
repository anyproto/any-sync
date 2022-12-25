package nodedebugrpc

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/secureservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/debug/nodedebugrpc/nodedebugrpcproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/nodespace"
	nodestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/node/storage"
	"storj.io/drpc"
)

const CName = "debug.nodedebugrpc"

var log = logger.NewNamed(CName)

func New() NodeDebugRpc {
	return &nodeDebugRpc{BaseDrpcServer: server.NewBaseDrpcServer()}
}

type NodeDebugRpc interface {
	app.ComponentRunnable
	drpc.Mux
}

type nodeDebugRpc struct {
	transport      secureservice.SecureService
	cfg            *config.Config
	treeCache      treegetter.TreeGetter
	spaceService   nodespace.Service
	storageService nodestorage.NodeStorage
	*server.BaseDrpcServer
}

func (s *nodeDebugRpc) Init(a *app.App) (err error) {
	s.treeCache = a.MustComponent(treegetter.CName).(treegetter.TreeGetter)
	s.spaceService = a.MustComponent(nodespace.CName).(nodespace.Service)
	s.storageService = a.MustComponent(spacestorage.CName).(nodestorage.NodeStorage)
	s.cfg = a.MustComponent(config.CName).(*config.Config)
	s.transport = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	return nil
}

func (s *nodeDebugRpc) Name() (name string) {
	return CName
}

func (s *nodeDebugRpc) Run(ctx context.Context) (err error) {
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
	return nodedebugrpcproto.DRPCRegisterNodeApi(s, &rpcHandler{s.treeCache, s.spaceService, s.storageService})
}

func (s *nodeDebugRpc) Close(ctx context.Context) (err error) {
	return s.BaseDrpcServer.Close(ctx)
}
