package ocache

import (
	"github.com/prometheus/client_golang/prometheus"
	"strings"
)

func WithPrometheus(reg *prometheus.Registry, namespace, subsystem string) Option {
	if subsystem == "" {
		subsystem = "cache"
	}
	nameSplit := strings.Split(namespace, ".")
	subSplit := strings.Split(subsystem, ".")
	namespace = strings.Join(nameSplit, "_")
	subsystem = strings.Join(subSplit, "_")
	if reg == nil {
		return nil
	}
	return func(cache *oCache) {
		cache.metrics = &metrics{
			hit: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "hit",
				Help:      "cache hit count",
			}),
			miss: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "miss",
				Help:      "cache miss count",
			}),
			gc: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "gc",
				Help:      "garbage collected count",
			}),
			size: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "size",
				Help:      "cache size",
			}, func() float64 {
				return float64(cache.Len())
			}),
		}
		reg.MustRegister(
			cache.metrics.hit,
			cache.metrics.miss,
			cache.metrics.gc,
			cache.metrics.size,
		)
	}
}

type metrics struct {
	hit  prometheus.Counter
	miss prometheus.Counter
	gc   prometheus.Counter
	size prometheus.GaugeFunc
}
