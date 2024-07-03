package sync

import "sync"

type guard struct {
	mu         sync.Mutex
	taken      map[string]struct{}
	limit      int
	takenCount int
}

func newGuard(limit int) *guard {
	return &guard{
		taken: make(map[string]struct{}),
		limit: limit,
	}
}

func (g *guard) TryTake(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.limit != 0 && g.takenCount >= g.limit {
		return false
	}
	if _, exists := g.taken[id]; exists {
		return false
	}
	g.takenCount++
	g.taken[id] = struct{}{}
	return true
}

func (g *guard) Release(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.takenCount--
	delete(g.taken, id)
}
