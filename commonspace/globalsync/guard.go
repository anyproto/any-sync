package globalsync

import "sync"

type Guard struct {
	mu         sync.Mutex
	taken      map[string]struct{}
	limit      int
	takenCount int
}

func NewGuard(limit int) *Guard {
	return &Guard{
		taken: make(map[string]struct{}),
		limit: limit,
	}
}

func (g *Guard) TryTake(id string) bool {
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

func (g *Guard) Release(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.takenCount--
	delete(g.taken, id)
}
