package headsync

import (
	"sync"
	"testing"

	"github.com/anyproto/any-sync/app/logger"
)

// TestSyncLogger_ConcurrentLogSyncDone guards against the data race that was
// possible once Sync could run concurrently (the periodic loop plus an
// on-demand DiffSync), both reaching logSyncDone on the same shared logger.
// Run with -race.
func TestSyncLogger_ConcurrentLogSyncDone(t *testing.T) {
	l := newSyncLogger(logger.NewNamed("test"), logPeriodSecs)
	const goroutines = 8
	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// differentIds == 0 forces the rate-limit branch that both
				// reads and writes lastLogged.
				l.logSyncDone("peer", 0, 0, 0, 0)
			}
		}()
	}
	wg.Wait()
}
