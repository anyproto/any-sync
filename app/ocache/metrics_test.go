package ocache

import (
	"context"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestWithPrometheus_MetricsConvertsDots(t *testing.T) {
	opt := WithPrometheus(prometheus.NewRegistry(), "some.name", "some.system")
	cache := New(func(ctx context.Context, id string) (value Object, err error) {
		return &testObject{}, nil
	}, opt).(*oCache)
	_, err := cache.Get(context.Background(), "id")
	require.NoError(t, err)
	require.True(t, strings.Contains(cache.metrics.hit.Desc().String(), "some_name_some_system_hit"))
}

func TestWithPrometheus_Registers(t *testing.T) {
	reg := prometheus.NewRegistry()
	cache := New(func(ctx context.Context, id string) (value Object, err error) {
		return &testObject{}, nil
	}, WithPrometheus(reg, "some.name", "some.system"))
	_, err := cache.Get(context.Background(), "id")
	require.NoError(t, err)
	families, err := reg.Gather()
	require.NoError(t, err)
	values := map[string]float64{}
	for _, mf := range families {
		m := mf.GetMetric()[0]
		if m.GetGauge() != nil {
			values[mf.GetName()] = m.GetGauge().GetValue()
		} else {
			values[mf.GetName()] = m.GetCounter().GetValue()
		}
	}
	require.Equal(t, map[string]float64{
		"some_name_some_system_hit":  0,
		"some_name_some_system_miss": 1,
		"some_name_some_system_gc":   0,
		"some_name_some_system_size": 1,
	}, values)
	// the same names cannot be registered twice: the reason a caller that
	// recreates its cache goes through NewPrometheusCollectors instead
	require.Panics(t, func() {
		New(nil, WithPrometheus(reg, "some.name", "some.system"))
	})
	require.NotPanics(t, func() {
		c := NewPrometheusCollectors("some.name", "some.system", func() int { return 0 })
		New(nil, c.Option())
		New(nil, c.Option())
	})
}
