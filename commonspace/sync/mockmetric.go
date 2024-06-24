package sync

import (
	"sync"

	"go.uber.org/zap"
)

type mockMetric struct {
	sync.Mutex
	totalSize uint64
}

func (m *mockMetric) UpdateQueueSize(size uint64, msgType int, add bool) {
	m.Lock()
	defer m.Unlock()
	if add {
		m.totalSize += size
	} else {
		m.totalSize -= size
	}
	log.Debug("total msg size", zap.Uint64("size", m.totalSize), zap.Uint64("added size", uint64(size)))
}
