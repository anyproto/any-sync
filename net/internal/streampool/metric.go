package streampool

import "github.com/prometheus/client_golang/prometheus"

func registerMetrics(ref *prometheus.Registry, sp *streamPool, name string) {
	ref.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: name,
			Subsystem: "streampool",
			Name:      "stream_count",
			Help:      "count of opened streams",
		}, func() float64 {
			sp.mu.Lock()
			defer sp.mu.Unlock()
			return float64(len(sp.streams))
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: name,
			Subsystem: "streampool",
			Name:      "tag_count",
			Help:      "count of active tags",
		}, func() float64 {
			sp.mu.Lock()
			defer sp.mu.Unlock()
			return float64(len(sp.streamIdsByTag))
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: name,
			Subsystem: "streampool",
			Name:      "dial_queue",
			Help:      "dial queue size",
		}, func() float64 {
			return float64(sp.dial.Batch.Len())
		}),
	)
}
