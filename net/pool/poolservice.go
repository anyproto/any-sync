package pool

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/dialer"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"time"
)

const (
	CName = "common.net.pool"
)

var log = logger.NewNamed(CName)

func New() Service {
	return &poolService{}
}

type Service interface {
	Pool
	app.ComponentRunnable
}

type poolService struct {
	// default pool
	*pool
	dialer    dialer.Dialer
	metricReg *prometheus.Registry
}

func (p *poolService) Init(a *app.App) (err error) {
	p.dialer = a.MustComponent(dialer.CName).(dialer.Dialer)
	p.pool = &pool{dialer: p.dialer}
	if m := a.Component(metric.CName); m != nil {
		p.metricReg = m.(metric.Metric).Registry()
	}
	p.pool.outgoing = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return p.dialer.Dial(ctx, id)
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Minute*5),
		ocache.WithPrometheus(p.metricReg, "netpool", "outgoing"),
	)
	p.pool.incoming = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return nil, ocache.ErrNotExists
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Minute*5),
		ocache.WithPrometheus(p.metricReg, "netpool", "incoming"),
	)
	return nil
}

func (p *pool) Run(ctx context.Context) (err error) {
	return nil
}

func (p *pool) Close(ctx context.Context) (err error) {
	if e := p.incoming.Close(); e != nil {
		log.Warn("close incoming cache error", zap.Error(e))
	}
	return p.outgoing.Close()
}
