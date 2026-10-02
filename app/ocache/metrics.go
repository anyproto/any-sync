package ocache

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

func WithPrometheus(reg *prometheus.Registry, namespace, subsystem string) Option {
	if reg == nil {
		return nil
	}
	return func(cache *oCache) {
		c := NewPrometheusCollectors(namespace, subsystem, cache.Len)
		c.MustRegister(reg)
		cache.metrics = &metrics{
			hit:  c.Hit,
			miss: c.Miss,
			gc:   c.GC,
			size: c.Size,
		}
	}
}

func WithPrometheusMetrics(hit, miss, gc prometheus.Counter, size prometheus.GaugeFunc) Option {
	return func(cache *oCache) {
		cache.metrics = &metrics{
			hit:  hit,
			miss: miss,
			gc:   gc,
			size: size,
		}
	}
}

// PrometheusCollectors are the collectors a cache reports through: the ones
// WithPrometheus builds and registers, exposed for a caller that recreates
// its cache and so must register them once and hand them to every instance.
type PrometheusCollectors struct {
	Hit, Miss, GC prometheus.Counter
	Size          prometheus.GaugeFunc
}

// NewPrometheusCollectors builds unregistered collectors with the names
// WithPrometheus would register (<namespace>_<subsystem>_{hit,miss,gc,size},
// dots turned into underscores, subsystem defaulting to "cache"). size is
// read through sizeFn, so it can resolve whichever cache is current.
func NewPrometheusCollectors(namespace, subsystem string, sizeFn func() int) PrometheusCollectors {
	if subsystem == "" {
		subsystem = "cache"
	}
	nameSplit := strings.Split(namespace, ".")
	subSplit := strings.Split(subsystem, ".")
	namespace = strings.Join(nameSplit, "_")
	subsystem = strings.Join(subSplit, "_")
	return PrometheusCollectors{
		Hit: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "hit",
			Help:      "cache hit count",
		}),
		Miss: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "miss",
			Help:      "cache miss count",
		}),
		GC: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc",
			Help:      "garbage collected count",
		}),
		Size: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "size",
			Help:      "cache size",
		}, func() float64 {
			return float64(sizeFn())
		}),
	}
}

// MustRegister registers the collectors with reg; like prometheus it panics
// on a second registration of the same names.
func (c PrometheusCollectors) MustRegister(reg prometheus.Registerer) {
	reg.MustRegister(c.Hit, c.Miss, c.GC, c.Size)
}

// Option makes a cache report through these collectors without registering
// anything.
func (c PrometheusCollectors) Option() Option {
	return WithPrometheusMetrics(c.Hit, c.Miss, c.GC, c.Size)
}

type metrics struct {
	hit  prometheus.Counter
	miss prometheus.Counter
	gc   prometheus.Counter
	size prometheus.GaugeFunc
}
