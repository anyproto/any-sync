package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type SyncMetric interface {
	SyncMetricState() SyncMetricState
}

type SyncMetricState struct {
	IncomingMsgCount  uint32
	IncomingMsgSize   uint64
	OutgoingMsgCount  uint32
	OutgoingMsgSize   uint64
	IncomingReqCount  uint32
	IncomingReqSize   uint64
	OutgoingReqCount  uint32
	OutgoingReqSize   uint64
	ReceivedRespCount uint32
	ReceivedRespSize  uint64
	SentRespCount     uint32
	SentRespSize      uint64
	TotalSize         uint64
}

func (st *SyncMetricState) Append(other SyncMetricState) {
	st.IncomingMsgSize += other.IncomingMsgSize
	st.OutgoingMsgSize += other.OutgoingMsgSize
	st.IncomingReqSize += other.IncomingReqSize
	st.OutgoingReqSize += other.OutgoingReqSize
	st.ReceivedRespSize += other.ReceivedRespSize
	st.SentRespSize += other.SentRespSize
	st.IncomingMsgCount += other.IncomingMsgCount
	st.OutgoingMsgCount += other.OutgoingMsgCount
	st.IncomingReqCount += other.IncomingReqCount
	st.OutgoingReqCount += other.OutgoingReqCount
	st.ReceivedRespCount += other.ReceivedRespCount
	st.SentRespCount += other.SentRespCount
	st.TotalSize += other.TotalSize
}

func (m *metric) getLastCached() SyncMetricState {
	m.mx.Lock()
	lastCached := m.lastCachedState
	if time.Now().Before(m.lastCachedDate.Add(m.lastCachedTimeout)) {
		m.mx.Unlock()
		return lastCached
	}
	var allMetrics []SyncMetric
	for _, sp := range m.syncMetrics {
		allMetrics = append(allMetrics, sp)
	}
	m.mx.Unlock()
	lastCached = SyncMetricState{}
	for _, mtr := range allMetrics {
		lastCached.Append(mtr.SyncMetricState())
	}
	m.mx.Lock()
	defer m.mx.Unlock()
	m.lastCachedState = lastCached
	m.lastCachedDate = time.Now()
	return lastCached
}

func (m *metric) registerSyncMetrics() error {
	gaugeFuncs := []prometheus.GaugeFunc{
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "incomingMsgSize",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().IncomingMsgSize)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "outgoingMsgSize",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().OutgoingMsgSize)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "outgoingReqSize",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().OutgoingReqSize)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "incomingReqSize",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().IncomingReqSize)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "receivedRespSize",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().ReceivedRespSize)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "sentRespSize",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().SentRespSize)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "incomingMsgCount",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().IncomingMsgCount)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "outgoingMsgCount",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().OutgoingMsgCount)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "outgoingReqCount",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().OutgoingReqCount)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "incomingReqCount",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().IncomingReqCount)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "receivedRespCount",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().ReceivedRespCount)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "sync",
			Subsystem: "space",
			Name:      "sentRespCount",
			Help:      "cache size",
		}, func() float64 {
			return float64(m.getLastCached().SentRespCount)
		}),
	}
	for _, gf := range gaugeFuncs {
		if err := m.registry.Register(gf); err != nil {
			return err
		}
	}
	return nil
}
