package peer

import (
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	// cleanupMaxWorkers bounds the sub-connection closes a peer runs at once
	cleanupMaxWorkers = 64
	// cleanupSlowClose is how long a single close may run before it is
	// logged as hung; nothing else happens to it
	cleanupSlowClose = time.Minute
)

// cleanupOwner closes a peer's sub connections off the callers' path. Closing
// a drpc conn waits for its reader, stream manager and transport, and a yamux
// stream close sends a FIN under a write timeout, so on a stalled connection a
// synchronous close turns a caller's expired deadline into a long hang.
//
// Workers are started on demand, up to cleanupMaxWorkers, and exit once there
// is nothing left to close, so an idle peer costs no goroutines. close never
// blocks and never drops a close; closes beyond the workers wait in a pending
// list.
//
// The pending list is not capped, and nothing escalates on a slow close (one
// running longer than cleanupSlowClose is only logged). The backlog is
// bounded in practice, not by construction:
//   - every close comes from a sub conn this peer opened, and the peer's open
//     limiter counts closes in flight (inFlight): past its threshold each
//     new open waits 100ms per extra conn, so opens settle at about 10/s per
//     peer while closes fall behind, and the list grows ever more slowly;
//   - each close is bounded by the transport: yamux stream closes by
//     StreamCloseTimeout and ConnectionWriteTimeout (both WriteTimeoutSec,
//     never 0), while QUIC, iroh and webtransport closes do not block;
//   - callers give up on their own deadlines rather than opening forever.
//
// Keepalive is not a bound: it can be disabled.
type cleanupOwner struct {
	peerId string

	mu      sync.Mutex
	pending []io.Closer
	workers int

	// inflight counts closes handed over and not yet finished
	inflight atomic.Int32

	// slowClose and onSlowClose are fields for tests
	slowClose   time.Duration
	onSlowClose func(cl io.Closer)
}

func newCleanupOwner(peerId string) *cleanupOwner {
	c := &cleanupOwner{peerId: peerId, slowClose: cleanupSlowClose}
	c.onSlowClose = func(io.Closer) {
		log.Warn("sub connection close is taking too long", zap.String("peerId", c.peerId), zap.Duration("after", c.slowClose))
	}
	return c
}

// close hands cl over to be closed in the background. It never blocks.
func (c *cleanupOwner) close(cl io.Closer) {
	if c == nil {
		// a peer built without NewPeer
		go func() { _ = cl.Close() }()
		return
	}
	c.inflight.Add(1)
	c.mu.Lock()
	if c.workers < cleanupMaxWorkers {
		c.workers++
		c.mu.Unlock()
		go c.work(cl)
		return
	}
	c.pending = append(c.pending, cl)
	c.mu.Unlock()
}

func (c *cleanupOwner) work(cl io.Closer) {
	for {
		c.closeOne(cl)
		c.inflight.Add(-1)
		c.mu.Lock()
		if len(c.pending) == 0 {
			c.workers--
			c.pending = nil
			c.mu.Unlock()
			return
		}
		cl = c.pending[0]
		c.pending[0] = nil
		c.pending = c.pending[1:]
		c.mu.Unlock()
	}
}

// closeOne closes cl, logging once if the close outlives slowClose
func (c *cleanupOwner) closeOne(cl io.Closer) {
	timer := time.AfterFunc(c.slowClose, func() { c.onSlowClose(cl) })
	_ = cl.Close()
	timer.Stop()
}

// inFlight returns the number of closes handed over and not yet finished
func (c *cleanupOwner) inFlight() int {
	if c == nil {
		return 0
	}
	return int(c.inflight.Load())
}

// stats returns the number of running and pending closes
func (c *cleanupOwner) stats() (running, pending int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.workers, len(c.pending)
}
