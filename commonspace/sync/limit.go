package sync

import (
	"sync"
)

type Limit struct {
	max    int
	tokens map[string]int
	cond   *sync.Cond
	mutex  sync.Mutex
}

func NewLimit(max int) *Limit {
	return &Limit{
		max:    max,
		tokens: make(map[string]int),
		cond:   sync.NewCond(&sync.Mutex{}),
	}
}

func (l *Limit) Take(id string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for l.tokens[id] >= l.max {
		l.cond.Wait()
	}

	l.tokens[id]++
}

func (l *Limit) Release(id string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.tokens[id] > 0 {
		l.tokens[id]--
		l.cond.Signal()
	}
}
