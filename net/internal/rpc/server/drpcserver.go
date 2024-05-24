package server

import (
	"context"
	"net"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/internal/rpc/limiter"
	rpc2 "github.com/anyproto/any-sync/net/rpc"

	"go.uber.org/zap"
	"storj.io/drpc"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"storj.io/drpc/drpcstream"
	"storj.io/drpc/drpcwire"
)

const CName = "common.net.drpcserver"

var log = logger.NewNamed(CName)

func New() DRPCServer {
	return &drpcServer{}
}

type DRPCServer interface {
	ServeConn(ctx context.Context, conn net.Conn) (err error)
	DrpcConfig() rpc2.Config
	app.Component
	drpc.Mux
}

type drpcServer struct {
	drpcServer *drpcserver.Server
	*drpcmux.Mux
	config  rpc2.Config
	metric  metric.Metric
	limiter limiter.RpcLimiter
}

type DRPCHandlerWrapper func(handler drpc.Handler) drpc.Handler

func (s *drpcServer) Name() (name string) {
	return CName
}

func (s *drpcServer) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(rpc2.ConfigGetter).GetDrpc()
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
	s.limiter, _ = a.Component(limiter.CName).(limiter.RpcLimiter)
	s.Mux = drpcmux.New()

	var handler drpc.Handler
	handler = s
	if s.limiter != nil {
		handler = s.limiter.WrapDRPCHandler(handler)
	}
	if s.metric != nil {
		handler = s.metric.WrapDRPCHandler(handler)
	}
	bufSize := s.config.Stream.MaxMsgSizeMb * (1 << 20)
	s.drpcServer = drpcserver.NewWithOptions(handler, drpcserver.Options{Manager: drpcmanager.Options{
		Reader: drpcwire.ReaderOptions{MaximumBufferSize: bufSize},
		Stream: drpcstream.Options{MaximumBufferSize: bufSize},
	}})
	return
}

func (s *drpcServer) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	l := log.With(zap.String("remoteAddr", conn.RemoteAddr().String())).With(zap.String("localAddr", conn.LocalAddr().String()))
	l.Debug("drpc serve peer")
	return s.drpcServer.ServeOne(ctx, conn)
}

func (s *drpcServer) DrpcConfig() rpc2.Config {
	return s.config
}
