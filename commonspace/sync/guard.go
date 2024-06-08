package sync

import "sync"

type guard struct {
	mu    sync.Mutex
	taken map[string]struct{}
}

func newGuard() *guard {
	return &guard{
		taken: make(map[string]struct{}),
	}
}

func (g *guard) TryTake(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.taken[id]; exists {
		return false
	}
	g.taken[id] = struct{}{}
	return true
}

func (g *guard) Release(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.taken, id)
}
