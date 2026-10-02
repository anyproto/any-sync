package peer

import (
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/net/transport"
)

const (
	// cleanupMaxWorkers bounds the sub-connection closes a peer runs at once
	cleanupMaxWorkers = 64
	// cleanupDefaultStallTimeout is the stall threshold for a transport that
	// does not report its write timeout: twice the default yamux one
	cleanupDefaultStallTimeout = 20 * time.Second
)

// closeStallTimeout is how long a single close may run before the transport
// counts as stalled. A healthy close takes about one round trip (drpc waits
// for the remote FIN), but on a congested link a FIN may wait up to the
// transport's write timeout, so the threshold is twice that. For yamux this
// is WriteTimeoutSec, which also sets ConnectionWriteTimeout and
// StreamCloseTimeout: a single stream close cannot legitimately outlast it.
func closeStallTimeout(mc transport.MultiConn) time.Duration {
	if wt, ok := mc.(transport.WriteTimeouter); ok {
		if d := wt.WriteTimeout(); d > 0 {
			return 2 * d
		}
	}
	return cleanupDefaultStallTimeout
}

// cleanupOwner closes a peer's sub connections off the callers' path. Closing
// a drpc conn waits for its reader, stream manager and transport, and a yamux
// stream close sends a FIN under a write timeout, so on a stalled connection a
// synchronous close turns a caller's expired deadline into a long hang.
//
// Workers are started on demand, up to cleanupMaxWorkers, and exit once there
// is nothing left to close, so an idle peer costs no goroutines. close never
// blocks and never drops a close; closes beyond the workers wait in a pending
// list. Its size is bounded by the peer's sub conns, and the peer's open
// limiter counts every close in flight (inFlight), so a peer that closes
// faster than the transport can keep up is throttled rather than piling up.
//
// Only on evidence of a stall, every worker busy and one of them on a close
// older than stallTimeout, is the whole MultiConn closed, which makes every
// pending and further close quick. A burst or a sustained rate of closes on a
// healthy connection just queues. The check runs when a close is handed over,
// so a stall is detected on the first close queued after stallTimeout;
// meanwhile the stuck closes are still bounded by the transport's own
// timeouts, so the cost of the delay is latency only.
type cleanupOwner struct {
	mc           transport.MultiConn
	stallTimeout time.Duration

	mu      sync.Mutex
	pending []io.Closer
	// running is the set of live workers
	running map[*cleanupWorker]struct{}

	// inflight counts closes handed over and not yet finished
	inflight    atomic.Int32
	escalated   atomic.Bool
	escalations atomic.Int64
}

type cleanupWorker struct {
	// started is when the current close began; guarded by cleanupOwner.mu
	started time.Time
}

func newCleanupOwner(mc transport.MultiConn) *cleanupOwner {
	return &cleanupOwner{
		mc:           mc,
		stallTimeout: closeStallTimeout(mc),
		running:      map[*cleanupWorker]struct{}{},
	}
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
	if len(c.running) < cleanupMaxWorkers {
		w := &cleanupWorker{started: time.Now()}
		c.running[w] = struct{}{}
		c.mu.Unlock()
		go c.work(w, cl)
		return
	}
	if !c.stalledLocked() {
		c.pending = append(c.pending, cl)
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	c.escalate(cl)
}

// stalledLocked reports whether a running close has outlived stallTimeout
func (c *cleanupOwner) stalledLocked() bool {
	now := time.Now()
	for w := range c.running {
		if now.Sub(w.started) > c.stallTimeout {
			return true
		}
	}
	return false
}

func (c *cleanupOwner) work(w *cleanupWorker, cl io.Closer) {
	for {
		_ = cl.Close()
		c.inflight.Add(-1)
		c.mu.Lock()
		if len(c.pending) == 0 {
			delete(c.running, w)
			c.pending = nil
			c.mu.Unlock()
			return
		}
		cl = c.pending[0]
		c.pending[0] = nil
		c.pending = c.pending[1:]
		w.started = time.Now()
		c.mu.Unlock()
	}
}

// escalate closes the whole connection: the transport is stalled, so the
// closes queued behind it would otherwise wait out its timeouts. A dead
// transport makes cl's close quick; the goroutines spawned here are bounded by
// the sub conns alive when the MultiConn closed, as no new ones can be opened
// afterwards.
func (c *cleanupOwner) escalate(cl io.Closer) {
	c.escalations.Add(1)
	first := c.escalated.CompareAndSwap(false, true)
	if first {
		log.Warn("sub connection cleanup is stalled: closing the connection")
	}
	go func() {
		if first {
			if err := c.mc.Close(); err != nil {
				log.Debug("close connection on stalled cleanup", zap.Error(err))
			}
		}
		_ = cl.Close()
		c.inflight.Add(-1)
	}()
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
	return len(c.running), len(c.pending)
}
