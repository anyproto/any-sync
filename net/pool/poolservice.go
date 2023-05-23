package pool

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/dialer"
	"github.com/prometheus/client_golang/prometheus"
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
	NewPool(name string) Pool
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
	p.pool.cache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return p.dialer.Dial(ctx, id)
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Minute*5),
		ocache.WithPrometheus(p.metricReg, "netpool", "default"),
	)
	return nil
}

func (p *poolService) NewPool(name string) Pool {
	return &pool{
		dialer: p.dialer,
		cache: ocache.New(
			func(ctx context.Context, id string) (value ocache.Object, err error) {
				return p.dialer.Dial(ctx, id)
			},
			ocache.WithLogger(log.Sugar()),
			ocache.WithGCPeriod(time.Minute),
			ocache.WithTTL(time.Minute*5),
			ocache.WithPrometheus(p.metricReg, "netpool", name),
		),
	}
}
