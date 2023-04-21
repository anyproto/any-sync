package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"storj.io/drpc"
	"time"
)

type prometheusDRPC struct {
	drpc.Handler
	SummaryVec *prometheus.SummaryVec
}

func (ph *prometheusDRPC) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	st := time.Now()
	defer func() {
		ph.SummaryVec.WithLabelValues(rpc).Observe(time.Since(st).Seconds())
	}()
	return ph.Handler.HandleRPC(stream, rpc)
}
