package metric

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
)

const CName = "common.metric"

var log = logger.NewNamed(CName)

func New() Metric {
	return &metric{
		lastCachedTimeout: time.Second * 10,
	}
}

type Metric interface {
	Registry() *prometheus.Registry
	WrapDRPCHandler(h drpc.Handler) drpc.Handler
	RequestLog(ctx context.Context, rpc string, fields ...zap.Field)
	RegisterSyncMetric(spaceId string, syncMetric SyncMetric)
	UnregisterSyncMetric(spaceId string, syncMetric SyncMetric)
	app.ComponentRunnable
}

type metric struct {
	registry          *prometheus.Registry
	rpcLog            logger.CtxLogger
	config            Config
	a                 *app.App
	appField          zap.Field
	mx                sync.Mutex
	syncMetrics       map[string]SyncMetric
	lastCachedState   SyncMetricState
	lastCachedDate    time.Time
	lastCachedTimeout time.Duration
}

func (m *metric) RegisterSyncMetric(spaceId string, syncMetric SyncMetric) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.syncMetrics[spaceId] = syncMetric
}

func (m *metric) UnregisterSyncMetric(spaceId string, syncMetric SyncMetric) {
	m.mx.Lock()
	defer m.mx.Unlock()
	delete(m.syncMetrics, spaceId)
}

func (m *metric) Init(a *app.App) (err error) {
	m.a = a
	m.syncMetrics = make(map[string]SyncMetric)
	m.registry = prometheus.NewRegistry()
	m.config = a.MustComponent("config").(configSource).GetMetric()
	m.rpcLog = logger.NewNamed("rpcLog")
	m.appField = App(a.Version())
	return nil
}

func (m *metric) Name() string {
	return CName
}

func (m *metric) Run(ctx context.Context) (err error) {
	if err = m.registry.Register(collectors.NewBuildInfoCollector()); err != nil {
		return err
	}
	if err = m.registry.Register(collectors.NewGoCollector()); err != nil {
		return err
	}
	if err = m.registry.Register(newVersionsCollector(m.a)); err != nil {
		return
	}
	if err = m.registerSyncMetrics(); err != nil {
		return
	}
	if m.config.Addr != "" {
		var errCh = make(chan error)
		http.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
		go func() {
			errCh <- http.ListenAndServe(m.config.Addr, nil)
		}()
		select {
		case err = <-errCh:
		case <-time.After(time.Second / 5):
		}
	}
	return
}

func (m *metric) Registry() *prometheus.Registry {
	return m.registry
}

func (m *metric) WrapDRPCHandler(h drpc.Handler) drpc.Handler {
	if m == nil {
		return h
	}
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
	if err := m.Registry().Register(histVec); err != nil {
		log.Warn("can't register prometheus drpc metric", zap.Error(err))
		return h
	}
	return &prometheusDRPC{
		Handler:    h,
		SummaryVec: histVec,
	}
}

func (m *metric) Close(ctx context.Context) (err error) {
	return
}
