package syncqueues

import "sync"

type Guard struct {
	mu    sync.Mutex
	taken map[string]struct{}
}

func NewGuard() *Guard {
	return &Guard{
		taken: make(map[string]struct{}),
	}
}

func (g *Guard) TryTake(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.taken[id]; exists {
		return false
	}
	g.taken[id] = struct{}{}
	return true
}

func (g *Guard) Release(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.taken, id)
}
