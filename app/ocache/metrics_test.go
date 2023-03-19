package ocache

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
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
