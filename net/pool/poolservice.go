package pool

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
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

type dialer interface {
	Dial(ctx context.Context, peerId string) (pr peer.Peer, err error)
}

type poolService struct {
	// default pool
	*pool
	dialer    dialer
	metricReg *prometheus.Registry
}

func (p *poolService) Init(a *app.App) (err error) {
	p.dialer = a.MustComponent("net.peerservice").(dialer)
	p.pool = &pool{}
	if m := a.Component(metric.CName); m != nil {
		p.metricReg = m.(metric.Metric).Registry()
	}
	p.pool.outgoing = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			if value, err = p.dialer.Dial(ctx, id); err != nil {
				if errors.Is(err, handshake.ErrIncompatibleVersion) {
					return &errObject{err: err, createdTime: atomic.NewTime(time.Now())}, nil
				}
			}
			return
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute/2),
		ocache.WithTTL(time.Minute),
		ocache.WithPrometheus(p.metricReg, "netpool", "outgoing"),
	)
	p.pool.incoming = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return nil, ocache.ErrNotExists
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute/2),
		ocache.WithTTL(time.Minute),
		ocache.WithPrometheus(p.metricReg, "netpool", "incoming"),
	)
	comp, ok := a.Component(debugstat.CName).(debugstat.StatService)
	if !ok {
		comp = debugstat.NewNoOp()
	}
	p.statService = comp
	p.statService.AddProvider(p)
	return nil
}

func (p *pool) Run(ctx context.Context) (err error) {
	return nil
}

func (p *pool) Close(ctx context.Context) (err error) {
	p.statService.RemoveProvider(p)
	if e := p.incoming.Close(); e != nil {
		log.Warn("close incoming cache error", zap.Error(e))
	}
	return p.outgoing.Close()
}

type errObject struct {
	err         error
	createdTime *atomic.Time
}

func (e *errObject) Error() error {
	return e.err
}

func (e *errObject) Close() (err error) {
	return
}

func (e *errObject) TryClose(_ time.Duration) (res bool, err error) {
	if e.createdTime.Load().Add(time.Minute * 20).Before(time.Now()) {
		return true, nil
	}
	return false, nil
}
