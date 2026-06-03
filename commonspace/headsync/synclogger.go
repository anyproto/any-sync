package headsync

import (
	"sync"
	"time"

	"github.com/anyproto/any-sync/app/logger"
	"go.uber.org/zap"
)

type syncLogger struct {
	mu          *sync.Mutex
	lastLogged  map[string]time.Time
	logInterval time.Duration
	logger.CtxLogger
}

func newSyncLogger(log logger.CtxLogger, syncLogPeriodSecs int) syncLogger {
	return syncLogger{
		mu:          &sync.Mutex{},
		lastLogged:  map[string]time.Time{},
		logInterval: time.Duration(syncLogPeriodSecs) * time.Second,
		CtxLogger:   log,
	}
}

func (s syncLogger) logSyncDone(peerId string, newIds, changedIds, removedIds, deltedIds int) {
	now := time.Now()
	differentIds := newIds + changedIds + removedIds + deltedIds
	s.mu.Lock()
	// always logging if some ids are different or there are no log interval
	if differentIds == 0 && s.logInterval > 0 {
		lastLogged := s.lastLogged[peerId]
		if now.Before(lastLogged.Add(s.logInterval)) {
			s.mu.Unlock()
			return
		}
	}
	s.lastLogged[peerId] = now
	s.mu.Unlock()
	s.Info("sync done:", zap.Int("newIds", newIds),
		zap.Int("changedIds", changedIds),
		zap.Int("removedIds", removedIds),
		zap.Int("already deleted ids", deltedIds),
		zap.String("peerId", peerId),
	)
}
