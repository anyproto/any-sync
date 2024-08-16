package sync

import (
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/metric"
)

type syncMetric struct {
	sync.Mutex
	incomingMsgCount  atomic.Int32
	incomingMsgSize   atomic.Int64
	outgoingMsgCount  atomic.Int32
	outgoingMsgSize   atomic.Int64
	incomingReqCount  atomic.Int32
	incomingReqSize   atomic.Int64
	outgoingReqCount  atomic.Int32
	outgoingReqSize   atomic.Int64
	receivedRespCount atomic.Int32
	receivedRespSize  atomic.Int64
	sentRespCount     atomic.Int32
	sentRespSize      atomic.Int64
	totalSize         atomic.Int64
}

func (m *syncMetric) SyncMetricState() metric.SyncMetricState {
	return metric.SyncMetricState{
		IncomingMsgCount:  uint32(m.incomingMsgCount.Load()),
		IncomingMsgSize:   uint64(m.incomingMsgSize.Load()),
		OutgoingMsgCount:  uint32(m.outgoingMsgCount.Load()),
		OutgoingMsgSize:   uint64(m.outgoingMsgSize.Load()),
		IncomingReqCount:  uint32(m.incomingReqCount.Load()),
		IncomingReqSize:   uint64(m.incomingReqSize.Load()),
		OutgoingReqCount:  uint32(m.outgoingReqCount.Load()),
		OutgoingReqSize:   uint64(m.outgoingReqSize.Load()),
		ReceivedRespCount: uint32(m.receivedRespCount.Load()),
		ReceivedRespSize:  uint64(m.receivedRespSize.Load()),
		SentRespCount:     uint32(m.sentRespCount.Load()),
		SentRespSize:      uint64(m.sentRespSize.Load()),
		TotalSize:         uint64(m.totalSize.Load()),
	}
}

func (m *syncMetric) UpdateQueueSize(size uint64, msgType int, add bool) {
	var (
		atSize  *atomic.Int64
		atCount *atomic.Int32
	)
	switch msgType {
	case syncdeps.MsgTypeIncoming:
		atSize = &m.incomingMsgSize
		atCount = &m.incomingMsgCount
	case syncdeps.MsgTypeIncomingRequest:
		atSize = &m.incomingReqSize
		atCount = &m.incomingReqCount
	case syncdeps.MsgTypeOutgoingRequest:
		atSize = &m.outgoingReqSize
		atCount = &m.outgoingReqCount
	case syncdeps.MsgTypeReceivedResponse:
		atSize = &m.receivedRespSize
		atCount = &m.receivedRespCount
	case syncdeps.MsgTypeSentResponse:
		atSize = &m.sentRespSize
		atCount = &m.sentRespCount
	default:
		return
	}
	intSize := int64(size)
	if add {
		atSize.Add(intSize)
		atCount.Add(1)
		m.totalSize.Add(intSize)
	} else {
		curCount := atCount.Load()
		curSize := atSize.Load()
		if curCount != 0 {
			atCount.Add(-1)
		}
		// TODO: fix the root cause :-)
		if curSize >= intSize {
			atSize.Add(-intSize)
			m.totalSize.Add(-intSize)
		} else {
			log.Error("syncMetric.UpdateQueueSize: totalSize is less than message size", zap.Int64("size", intSize), zap.Int64("totalSize", curSize))
		}
	}
}
