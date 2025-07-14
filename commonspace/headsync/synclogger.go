package headsync

import (
	"time"

	"github.com/anyproto/any-sync/app/logger"
)

type syncLogger struct {
	lastLogged  map[string]time.Time
	logInterval time.Duration
	logger.CtxLogger
}

func newSyncLogger(log logger.CtxLogger, syncLogPeriodSecs int) syncLogger {
	return syncLogger{
		lastLogged:  map[string]time.Time{},
		logInterval: time.Duration(syncLogPeriodSecs) * time.Second,
		CtxLogger:   log,
	}
}

func (s syncLogger) logSyncDone(peerId string, newIds, changedIds, removedIds, deltedIds int) {
	now := time.Now()
	differentIds := newIds + changedIds + removedIds + deltedIds
	// always logging if some ids are different or there are no log interval
	if differentIds == 0 && s.logInterval > 0 {
		lastLogged := s.lastLogged[peerId]
		if now.Before(lastLogged.Add(s.logInterval)) {
			return
		}
	}
	s.lastLogged[peerId] = now
}
