package pool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/peer"
)

// The hit path: the peer is already pooled, no Flush runs, a prometheus
// registry is attached as on the servers. Comparable across branches, so the
// helpers below depend only on the fixtures that exist on main (testPeer,
// dialerMock).

type benchMetric struct {
	metric.Metric
	reg *prometheus.Registry
}

func (m *benchMetric) Init(a *app.App) error           { m.reg = prometheus.NewRegistry(); return nil }
func (m *benchMetric) Name() string                    { return metric.CName }
func (m *benchMetric) Run(ctx context.Context) error   { return nil }
func (m *benchMetric) Close(ctx context.Context) error { return nil }
func (m *benchMetric) Registry() *prometheus.Registry  { return m.reg }

// benchPeer never closes itself through TryClose, so the GC cannot evict it
// mid-benchmark, and its Close is idempotent like the real peer's
type benchPeer struct {
	*testPeer
	once sync.Once
}

func newBenchPeer(id string) *benchPeer {
	return &benchPeer{testPeer: newTestPeer(id)}
}

func (p *benchPeer) Close() error {
	p.once.Do(func() { _ = p.testPeer.Close() })
	return nil
}

func (p *benchPeer) TryClose(time.Duration) (bool, error) { return false, nil }

func benchPool(b *testing.B, op func(ctx context.Context, s Service) error) {
	s := New()
	a := new(app.App)
	a.Register(s)
	a.Register(&dialerMock{dial: func(ctx context.Context, id string) (peer.Peer, error) {
		return newBenchPeer(id), nil
	}})
	a.Register(&benchMetric{})
	if err := a.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = a.Close(context.Background()) }()
	ctx := context.Background()
	if _, err := s.Get(ctx, "out"); err != nil {
		b.Fatal(err)
	}
	if err := s.AddPeer(ctx, newBenchPeer("in")); err != nil {
		b.Fatal(err)
	}
	b.Run("serial", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := op(ctx, s); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := op(ctx, s); err != nil {
					b.Error(err)
					return
				}
			}
		})
	})
}

func BenchmarkPool_GetIncoming(b *testing.B) {
	benchPool(b, func(ctx context.Context, s Service) error { _, err := s.Get(ctx, "in"); return err })
}

func BenchmarkPool_GetOutgoing(b *testing.B) {
	benchPool(b, func(ctx context.Context, s Service) error { _, err := s.Get(ctx, "out"); return err })
}

func BenchmarkPool_PickIncoming(b *testing.B) {
	benchPool(b, func(ctx context.Context, s Service) error { _, err := s.Pick(ctx, "in"); return err })
}

func BenchmarkPool_PickOutgoing(b *testing.B) {
	benchPool(b, func(ctx context.Context, s Service) error { _, err := s.Pick(ctx, "out"); return err })
}

func BenchmarkPool_GetOneOf(b *testing.B) {
	ids := []string{"x1", "x2", "out"}
	benchPool(b, func(ctx context.Context, s Service) error { _, err := s.GetOneOf(ctx, ids); return err })
}

// a Pick for a peer that is not pooled: servers probe connectivity this way
func BenchmarkPool_PickMiss(b *testing.B) {
	benchPool(b, func(ctx context.Context, s Service) error {
		if _, err := s.Pick(ctx, "absent"); err == nil {
			return fmt.Errorf("unexpected hit")
		}
		return nil
	})
}

// servers pass request contexts, which are cancellable
func BenchmarkPool_GetIncomingCancellableCtx(b *testing.B) {
	rctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	benchPool(b, func(_ context.Context, s Service) error { _, err := s.Get(rctx, "in"); return err })
}
