package keyvalue

import (
	"context"
	"sync"
)

type concurrentLimiter struct {
	mu         sync.Mutex
	inProgress map[string]bool
	wg         sync.WaitGroup
}

func newConcurrentLimiter() *concurrentLimiter {
	return &concurrentLimiter{
		inProgress: make(map[string]bool),
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

func (cl *concurrentLimiter) Close() {
	cl.wg.Wait()
}
