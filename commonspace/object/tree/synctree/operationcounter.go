package synctree

import (
	"sync"
	"time"
)

type operationCounter struct {
	value    int32
	lastOp   time.Time
	lastOpMu sync.Mutex
	ch       chan struct{}
	limit    time.Duration
}

func defaultOpCounter() *operationCounter {
	return newOpCounter(50 * time.Second)
}

func newOpCounter(limit time.Duration) *operationCounter {
	ac := &operationCounter{
		limit: limit,
		ch:    make(chan struct{}, 1),
	}
	return ac
}

func (ac *operationCounter) Increment() {
	ac.lastOpMu.Lock()
	ac.value++
	ac.lastOp = time.Now()
	ac.lastOpMu.Unlock()
	select {
	case ac.ch <- struct{}{}:
	default:
	}
}

func (ac *operationCounter) Decrement() {
	ac.lastOpMu.Lock()
	ac.value--
	ac.lastOp = time.Now()
	ac.lastOpMu.Unlock()
	select {
	case ac.ch <- struct{}{}:
	default:
	}
}

func (ac *operationCounter) WaitIdle() {
	for {
		select {
		case <-ac.ch:
		case <-time.After(ac.limit):
			ac.lastOpMu.Lock()
			if ac.value == 0 {
				ac.lastOpMu.Unlock()
				return
			}
			ac.lastOpMu.Unlock()
		}
	}
}
