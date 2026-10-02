package peer

import (
	"time"
)

type limiter struct {
	startThreshold int
	slowDownStep   time.Duration
}

func (l limiter) wait(count int) <-chan time.Time {
	if d := l.delay(count); d > 0 {
		return time.After(d)
	}
	return nil
}

// delay is how long to hold off a new conn when count are already open
func (l limiter) delay(count int) time.Duration {
	if count > l.startThreshold {
		return l.slowDownStep * time.Duration(count-l.startThreshold)
	}
	return 0
}
