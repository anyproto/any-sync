package pubsub

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const rateLimiterIdleTimeout = time.Minute

// peerRateLimiter is a per-peer publish token bucket with lazy garbage collection
// of idle entries, so memory stays O(recently active peers).
type peerRateLimiter struct {
	mu     sync.Mutex
	peers  map[string]*peerRate
	rps    rate.Limit
	burst  int
	lastGC time.Time
}

type peerRate struct {
	*rate.Limiter
	lastUsage time.Time
}

func newPeerRateLimiter(rps float64, burst int) *peerRateLimiter {
	return &peerRateLimiter{
		peers:  make(map[string]*peerRate),
		rps:    rate.Limit(rps),
		burst:  burst,
		lastGC: time.Now(),
	}
}

func (r *peerRateLimiter) allow(peerId string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if now.Sub(r.lastGC) > rateLimiterIdleTimeout {
		for id, pr := range r.peers {
			if now.Sub(pr.lastUsage) > rateLimiterIdleTimeout {
				delete(r.peers, id)
			}
		}
		r.lastGC = now
	}
	pr, ok := r.peers[peerId]
	if !ok {
		pr = &peerRate{Limiter: rate.NewLimiter(r.rps, r.burst)}
		r.peers[peerId] = pr
	}
	pr.lastUsage = now
	return pr.Allow()
}
