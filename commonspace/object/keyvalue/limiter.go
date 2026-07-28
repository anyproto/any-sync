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
	closed       bool
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
	// a bounded Close can return while wg still has waiters parked: an Add after
	// that panics with "WaitGroup is reused before previous Wait has returned",
	// and SyncWithPeer stays callable after Close
	if cl.closed || cl.inProgress[id] {
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

// Close rejects further requests and waits for the scheduled ones to finish. The
// wait is bounded: a request already past the ctx check is doing peer rpc that may
// not return while nodes are unreachable, and blocking here forever wedges the
// whole app.Close. On timeout the request is abandoned, not stopped.
func (cl *concurrentLimiter) Close(ctx context.Context) {
	cl.mu.Lock()
	cl.closed = true
	cl.mu.Unlock()
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
