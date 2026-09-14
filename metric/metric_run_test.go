package metric

import (
	"context"
	"testing"

	"github.com/anyproto/any-sync/app"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRun_DoesNotPanicWithMultipleInstances(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("metric.Run should not panic with multiple metric instances: %v", r)
		}
	}()

	first := newTestMetric()
	if err := first.Run(context.Background()); err == nil {
		t.Fatal("first metric run should return listen error with invalid test address")
	}

	second := newTestMetric()
	if err := second.Run(context.Background()); err == nil {
		t.Fatal("second metric run should return listen error with invalid test address")
	}
}

func newTestMetric() *metric {
	return &metric{
		registry: prometheus.NewRegistry(),
		// Intentionally invalid address to avoid creating long-lived listeners in unit tests.
		config:      Config{Addr: "127.0.0.1"},
		a:           &app.App{},
		syncMetrics: map[string]SyncMetric{},
	}
}
