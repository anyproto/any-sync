package peer

import (
	"time"
)

type limiter struct {
	startThreshold int
	slowDownStep   time.Duration
}

func (l limiter) wait(count int) <-chan time.Time {
	if count > l.startThreshold {
		wait := l.slowDownStep * time.Duration(count-l.startThreshold)
		return time.After(wait)
	}
	return nil
}
