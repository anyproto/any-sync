package clientdebugrpc

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/debug/clientdebugrpc/clientdebugrpcproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document"
	clientstorage "github.com/anytypeio/go-anytype-infrastructure-experiments/client/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/secureservice"
	"storj.io/drpc"
)

const CName = "common.debug.clientdebugrpc"

var log = logger.NewNamed(CName)

func New() ClientDebugRpc {
	return &service{BaseDrpcServer: server.NewBaseDrpcServer()}
}

type configGetter interface {
	GetDebugNet() net.Config
}

type ClientDebugRpc interface {
	app.ComponentRunnable
	drpc.Mux
}

type service struct {
	transport      secureservice.SecureService
	cfg            net.Config
	spaceService   clientspace.Service
	storageService clientstorage.ClientStorage
	docService     document.Service
	account        accountservice.Service
	file           fileservice.FileService
	*server.BaseDrpcServer
}

func (s *service) Init(a *app.App) (err error) {
	s.spaceService = a.MustComponent(clientspace.CName).(clientspace.Service)
	s.storageService = a.MustComponent(spacestorage.CName).(clientstorage.ClientStorage)
	s.docService = a.MustComponent(document.CName).(document.Service)
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.cfg = a.MustComponent("config").(configGetter).GetDebugNet()
	s.transport = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	s.file = a.MustComponent(fileservice.CName).(fileservice.FileService)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	params := server.Params{
		BufferSizeMb:  s.cfg.Stream.MaxMsgSizeMb,
		TimeoutMillis: s.cfg.Stream.TimeoutMilliseconds,
		ListenAddrs:   s.cfg.Server.ListenAddrs,
		Wrapper: func(handler drpc.Handler) drpc.Handler {
			return handler
		},
		Converter: s.transport.BasicListener,
	}
	err = s.BaseDrpcServer.Run(ctx, params)
	if err != nil {
		return
	}
	return clientdebugrpcproto.DRPCRegisterClientApi(s, &rpcHandler{
		spaceService:   s.spaceService,
		storageService: s.storageService,
		docService:     s.docService,
		account:        s.account,
		file:           s.file,
	})
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.BaseDrpcServer.Close(ctx)
}
