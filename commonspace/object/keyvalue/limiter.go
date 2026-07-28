package keyvalue

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

const limiterCloseTimeout = time.Second * 10

type concurrentLimiter struct {
	mu           sync.Mutex
	inProgress   map[string]bool
	wg           sync.WaitGroup
	closeTimeout time.Duration
}

func newConcurrentLimiter() *concurrentLimiter {
	return &concurrentLimiter{
		inProgress:   make(map[string]bool),
		closeTimeout: limiterCloseTimeout,
	}
}

func (cl *concurrentLimiter) ScheduleRequest(ctx context.Context, id string, action func()) bool {
	cl.mu.Lock()
	if cl.inProgress[id] {
		cl.mu.Unlock()
		return false
	}

	cl.inProgress[id] = true
	cl.wg.Add(1)
	cl.mu.Unlock()

	go func() {
		defer func() {
			cl.mu.Lock()
			delete(cl.inProgress, id)
			cl.mu.Unlock()
			cl.wg.Done()
		}()

		select {
		case <-ctx.Done():
			return
		default:
			action()
		}
	}()

	return true
}

// Close waits for the scheduled requests to finish. The wait is bounded: a request
// already past the ctx check is doing peer rpc that may not return while nodes are
// unreachable, and blocking here forever wedges the whole app.Close.
func (cl *concurrentLimiter) Close(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		cl.wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(cl.closeTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-ctx.Done():
		log.WarnCtx(ctx, "key value close interrupted, peer sync is still in flight", zap.Error(ctx.Err()))
	case <-timer.C:
		log.WarnCtx(ctx, "key value close timed out, peer sync is still in flight")
	}
}
