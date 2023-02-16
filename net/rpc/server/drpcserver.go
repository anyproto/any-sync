package server

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/metric"
	anyNet "github.com/anytypeio/any-sync/net"
	"github.com/anytypeio/any-sync/net/secureservice"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/prometheus/client_golang/prometheus"
	"net"
	"storj.io/drpc"
	"time"
)

const CName = "common.net.drpcserver"

var log = logger.NewNamed(CName)

func New() DRPCServer {
	return &drpcServer{BaseDrpcServer: NewBaseDrpcServer()}
}

type DRPCServer interface {
	app.ComponentRunnable
	drpc.Mux
}

type drpcServer struct {
	config    anyNet.Config
	metric    metric.Metric
	transport secureservice.SecureService
	*BaseDrpcServer
}

func (s *drpcServer) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(anyNet.ConfigGetter).GetNet()
	s.metric = a.MustComponent(metric.CName).(metric.Metric)
	s.transport = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	return nil
}

func (s *drpcServer) Name() (name string) {
	return CName
}

func (s *drpcServer) Run(ctx context.Context) (err error) {
	histVec := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "drpc",
		Subsystem: "server",
		Name:      "duration_seconds",
		Objectives: map[float64]float64{
			0.5:  0.5,
			0.85: 0.01,
			0.95: 0.0005,
			0.99: 0.0001,
		},
	}, []string{"rpc"})
	if err = s.metric.Registry().Register(histVec); err != nil {
		return
	}
	params := Params{
		BufferSizeMb:  s.config.Stream.MaxMsgSizeMb,
		TimeoutMillis: s.config.Stream.TimeoutMilliseconds,
		ListenAddrs:   s.config.Server.ListenAddrs,
		Wrapper: func(handler drpc.Handler) drpc.Handler {
			return &metric.PrometheusDRPC{
				Handler:    handler,
				SummaryVec: histVec,
			}
		},
	}
	s.handshake = func(conn net.Conn) (cCtx context.Context, sc sec.SecureConn, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		return s.transport.SecureInbound(ctx, conn)
	}
	return s.BaseDrpcServer.Run(ctx, params)
}

func (s *drpcServer) Close(ctx context.Context) (err error) {
	return s.BaseDrpcServer.Close(ctx)
}
