package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"storj.io/drpc"
	"time"
	"unicode/utf8"
)

type prometheusDRPC struct {
	drpc.Handler
	SummaryVec *prometheus.SummaryVec
}

func (ph *prometheusDRPC) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	st := time.Now()
	defer func() {
		if utf8.ValidString(rpc) {
			ph.SummaryVec.WithLabelValues(rpc).Observe(time.Since(st).Seconds())
		} else {
			log.WarnCtx(stream.Context(), "invalid rpc string", zap.String("rpc", rpc))
		}
	}()
	return ph.Handler.HandleRPC(stream, rpc)
}
