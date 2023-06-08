package server

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/rpc"
	"go.uber.org/zap"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"storj.io/drpc/drpcwire"
)

const CName = "common.net.drpcserver"

var log = logger.NewNamed(CName)

func New() DRPCServer {
	return &drpcServer{}
}

type DRPCServer interface {
	ServeConn(ctx context.Context, conn net.Conn) (err error)
	app.Component
	drpc.Mux
}

type drpcServer struct {
	drpcServer *drpcserver.Server
	*drpcmux.Mux
	config rpc.Config
	metric metric.Metric
}

type DRPCHandlerWrapper func(handler drpc.Handler) drpc.Handler

func (s *drpcServer) Name() (name string) {
	return CName
}

func (s *drpcServer) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(rpc.ConfigGetter).GetDrpc()
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
	s.Mux = drpcmux.New()

	var handler drpc.Handler
	handler = s
	if s.metric != nil {
		handler = s.metric.WrapDRPCHandler(s)
	}
	s.drpcServer = drpcserver.NewWithOptions(handler, drpcserver.Options{Manager: drpcmanager.Options{
		Reader: drpcwire.ReaderOptions{MaximumBufferSize: s.config.Stream.MaxMsgSizeMb * (1 << 20)},
	}})
	return
}

func (s *drpcServer) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	l := log.With(zap.String("remoteAddr", conn.RemoteAddr().String())).With(zap.String("localAddr", conn.LocalAddr().String()))
	l.Debug("drpc serve peer")
	return s.drpcServer.ServeOne(ctx, conn)
}
