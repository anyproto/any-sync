package sync

import (
	"sync"
)

type Limit struct {
	max    int
	tokens map[string]int
	mx     sync.Mutex
}

func NewLimit(max int) *Limit {
	return &Limit{
		max:    max,
		tokens: make(map[string]int),
	}
}

func (l *Limit) Take(id string) bool {
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.tokens[id] >= l.max {
		return false
	}
	l.tokens[id]++
	return true
}

func (l *Limit) Release(id string) {
	l.mx.Lock()
	defer l.mx.Unlock()
	if l.tokens[id] > 0 {
		l.tokens[id]--
	}
}
