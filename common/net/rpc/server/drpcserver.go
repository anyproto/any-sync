package server

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/metric"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/secure"
	"github.com/prometheus/client_golang/prometheus"
	"storj.io/drpc"
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

type configGetter interface {
	GetGRPCServer() config.GrpcServer
}

type drpcServer struct {
	config    config.GrpcServer
	metric    metric.Metric
	transport secure.Service
	*BaseDrpcServer
}

func (s *drpcServer) Init(a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(configGetter).GetGRPCServer()
	s.metric = a.MustComponent(metric.CName).(metric.Metric)
	s.transport = a.MustComponent(secure.CName).(secure.Service)
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
	return s.BaseDrpcServer.Run(ctx,
		s.config.ListenAddrs,
		func(handler drpc.Handler) drpc.Handler {
			return &metric.PrometheusDRPC{
				Handler:    handler,
				SummaryVec: histVec,
			}
		},
		s.transport.TLSListener)
}

func (s *drpcServer) Close(ctx context.Context) (err error) {
	return s.BaseDrpcServer.Close(ctx)
}
