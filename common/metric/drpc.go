package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"storj.io/drpc"
	"time"
)

type PrometheusDRPC struct {
	drpc.Handler
	HistogramVec *prometheus.HistogramVec
}

func (ph *PrometheusDRPC) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	st := time.Now()
	defer func() {
		ph.HistogramVec.WithLabelValues(rpc).Observe(time.Since(st).Seconds())
	}()
	return ph.Handler.HandleRPC(stream, rpc)
}
