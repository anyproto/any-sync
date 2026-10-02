package pool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peerobserver"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
)

const (
	CName = "common.net.pool"
)

// closeTimeout bounds a cache close pass and Close's wait on the pairs an
// earlier Flush is still closing (see pool.closeTimeout)
const closeTimeout = 10 * time.Second

var log = logger.NewNamed(CName)

func New() Service {
	return &poolService{pool: &pool{closeTimeout: closeTimeout}}
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
	if p.pool.closeTimeout <= 0 {
		p.pool.closeTimeout = closeTimeout
	}
	p.pool.closingCtx, p.pool.closingCancel = context.WithCancel(context.Background())
	if m := a.Component(metric.CName); m != nil {
		p.metricReg = m.(metric.Metric).Registry()
	}
	// Flush recreates the caches, so the collectors are built and registered
	// once here and shared by every instance (WithPrometheus would register
	// the same names again and panic). The names stay
	// netpool_{outgoing,incoming}_{hit,miss,gc,size}; size reads the current
	// cache, so it is registered only once a pair is published. ocache skips a
	// nil option.
	var outgoing, incoming ocache.PrometheusCollectors
	var outgoingMetrics, incomingMetrics ocache.Option
	if p.metricReg != nil {
		outgoing = ocache.NewPrometheusCollectors("netpool", "outgoing", func() int {
			return p.pool.current.Load().outgoing.Len()
		})
		incoming = ocache.NewPrometheusCollectors("netpool", "incoming", func() int {
			return p.pool.current.Load().incoming.Len()
		})
		outgoingMetrics, incomingMetrics = outgoing.Option(), incoming.Option()
		p.pool.incomingMiss = incoming.Miss
	}
	p.pool.newCaches = func() *caches {
		return p.newCaches(outgoingMetrics, incomingMetrics)
	}
	p.pool.current.Store(p.pool.newCaches())
	if p.metricReg != nil {
		outgoing.MustRegister(p.metricReg)
		incoming.MustRegister(p.metricReg)
	}
	comp, ok := a.Component(debugstat.CName).(debugstat.StatService)
	if !ok {
		comp = debugstat.NewNoOp()
	}
	p.statService = comp
	p.statService.AddProvider(p)
	p.pool.observer = peerobserver.FromApp(a)
	return nil
}

// newCaches builds one cache pair. The outgoing loader binds its watcher to
// the cache it loads into, not to whichever pair is current when the dial
// finishes: after a Flush that is a different one.
func (p *poolService) newCaches(outgoingMetrics, incomingMetrics ocache.Option) *caches {
	c := &caches{}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.outgoing = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			value, err = p.dialer.Dial(ctx, id)
			if err != nil {
				if errors.Is(err, handshake.ErrIncompatibleVersion) {
					return &errObject{id: id, err: err, createdTime: atomic.NewTime(time.Now())}, nil
				}
				return value, err
			}
			if pr, ok := value.(peer.Peer); ok {
				go p.pool.evictOnClose(pr, c.outgoing, false)
			}
			return value, nil
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute/2),
		ocache.WithTTL(time.Minute),
		ocache.WithCloseTimeout(p.pool.closeTimeout),
		outgoingMetrics,
	)
	c.incoming = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return nil, ocache.ErrNotExists
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute/2),
		ocache.WithTTL(time.Minute),
		ocache.WithCloseTimeout(p.pool.closeTimeout),
		incomingMetrics,
	)
	c.peekIncoming, c.peekOutgoing = mustPeeker(c.incoming), mustPeeker(c.outgoing)
	return c
}

// mustPeeker asserts the hit-path read on a cache; every cache the pool builds
// comes from ocache.New, so a failure is a programming error
func mustPeeker(c ocache.OCache) ocache.Peeker {
	pk, ok := c.(ocache.Peeker)
	if !ok {
		panic(fmt.Sprintf("pool: cache %T does not implement ocache.Peeker", c))
	}
	return pk
}

func (p *pool) Run(ctx context.Context) (err error) {
	return nil
}

// Close closes the current pair, waits for its peer teardowns and for the
// pairs earlier flushes are still closing, all bounded by ctx and closeTimeout;
// on a timeout the teardown goroutine keeps running in the background (it
// ends when the hung peer close does, which may be never) and Close returns
// nil. Flush is a no-op from here on. Idempotent.
func (p *pool) Close(ctx context.Context) (err error) {
	p.swapMu.Lock()
	if p.closed {
		p.swapMu.Unlock()
		return nil
	}
	p.closed = true
	p.swapMu.Unlock()
	if p.closingCancel != nil {
		p.closingCancel()
	}
	p.statService.RemoveProvider(p)
	cur := p.current.Load()
	// lookups blocked on the current pair fail now with ErrClosed (see lookup)
	cur.cancel()
	done := make(chan error, 1)
	go func() {
		peers, err := closeCaches(cur)
		peers.Wait()
		p.closing.Wait()
		done <- err
	}()
	timer := time.NewTimer(p.closeTimeout)
	defer timer.Stop()
	select {
	case err = <-done:
	case <-ctx.Done():
		log.Warn("pool close: ctx done before every peer closed")
	case <-timer.C:
		log.Warn("pool close: timed out waiting for peers to close")
	}
	return err
}

type errObject struct {
	id          string
	err         error
	createdTime *atomic.Time
}

func (e *errObject) Error() error {
	return e.err
}

// keepOnFlush reports whether Flush carries this cached error over into the
// fresh cache. An incompatible-version verdict survives: it says nothing
// about the network, and dropping it would defeat its 20-minute backoff on
// every recovery. Only published verdicts are carried; one whose dial is
// still in flight at the swap stays with the old pair (one extra dial).
func (e *errObject) keepOnFlush() bool {
	return errors.Is(e.err, handshake.ErrIncompatibleVersion)
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
