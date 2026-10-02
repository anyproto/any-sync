//go:generate mockgen -destination mock_pool/mock_pool.go github.com/anyproto/any-sync/net/pool Pool,Service
package pool

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peerobserver"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
)

// Pool creates and caches outgoing connection
type Pool interface {
	// Get lookups to peer in existing connections or creates and outgoing new one
	Get(ctx context.Context, id string) (peer.Peer, error)
	// GetOneOf searches at least one existing connection in outgoing or creates a new one from a randomly selected id from given list
	GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error)
	// AddPeer adds incoming peer to the pool. The pool evicts peers by
	// instance (ocache.RemoveSame), so peer.Peer implementations must be
	// comparable (pointers)
	AddPeer(ctx context.Context, p peer.Peer) (err error)
	// Pick checks if a connection with the peer exists, without dialing.
	// For a peer whose last dial failed it returns the cached dial error.
	Pick(ctx context.Context, id string) (pr peer.Peer, err error)
	// Flush removes all connections from the pool
	Flush(ctx context.Context) error
}

type poolStats struct {
	PeerStats []*peer.Stat `json:"peerStats"`
}

// caches is one immutable pair of peer caches. Flush replaces the whole pair,
// so a lookup that snapshots it never mixes a new incoming cache with an old
// outgoing one.
type caches struct {
	incoming ocache.OCache
	outgoing ocache.OCache
	// the same two caches as ocache.Peeker, asserted once at build (ocache.New
	// always returns one) so the hit path does no type assertion
	peekIncoming ocache.Peeker
	peekOutgoing ocache.Peeker
	// ctx is cancelled the moment the pair stops being current (swap or pool
	// Close), before its caches close; lookup merges it into the caller's ctx
	// so a lookup blocked on this pair (a dial, a wait behind a GC TryClose)
	// is cut short and retried on the current one
	ctx    context.Context
	cancel context.CancelFunc
}

type pool struct {
	// current is the published cache pair. Lookups load it once per call and
	// re-check it afterwards (see lookup); Flush swaps it under swapMu and
	// closes the replaced pair in the background; Close leaves it in place,
	// closed, so lookups on a closed pool fail with ocache.ErrClosed.
	current atomic.Pointer[caches]
	// swapMu orders the swap against AddPeer (read side, see addIncoming) and
	// against Close; closed is set under it, after which Flush is a no-op
	swapMu sync.RWMutex
	closed bool
	// closing counts the replaced pairs whose teardown (caches and peers) is
	// still running in the background; Close waits for them, bounded
	closing sync.WaitGroup
	// newCaches builds a fresh pair with its own ctx; set by the service at Init
	newCaches func() *caches
	// closeTimeout bounds the ocache close passes and Close as a whole
	closeTimeout time.Duration
	// incomingMiss is the incoming cache's miss counter (nil without a
	// registry): Get counts a miss there when the fast path finds the peer
	// in outgoing, as a Get through the caches would
	incomingMiss prometheus.Counter

	statService   debugstat.StatService
	closingCtx    context.Context
	closingCancel context.CancelFunc
	// observer is bound once at Init (peerobserver.CName) and never changes
	// afterwards, so it is read without locking; its zero value is a no-op
	observer peerobserver.Notifier
}

func (p *pool) Name() (name string) {
	return CName
}

// lookup runs f against the current pair and repeats it while the pair is
// swapped underneath: once Flush has published a new pair, a result from the
// replaced one is a pre-flush peer, or an error from a dial the swap cut
// short, and neither may reach the caller. f gets a ctx that is also
// cancelled by the swap, so a lookup blocked on the old pair does not wait
// out a dead dial. One retry is not enough under back-to-back flushes, so
// this loops until a result comes from the pair that is still current
// afterwards. Bounded by ctx: if flushes keep coming faster than a dial
// completes no lookup can succeed, and the caller gets its ctx error (the
// recovery worker does not flush in a tight loop). ErrClosed reaches the
// caller only when the pool itself is closed.
func (p *pool) lookup(ctx context.Context, f func(ctx context.Context, c *caches) (peer.Peer, error)) (peer.Peer, error) {
	for {
		c := p.current.Load()
		pr, err := func() (peer.Peer, error) {
			// deferred so a loader panic re-raised by ocache does not leave
			// the registration on the pair ctx behind
			lctx, cancel := context.WithCancel(ctx)
			defer cancel()
			defer context.AfterFunc(c.ctx, cancel)()
			return f(lctx, c)
		}()
		// read before re-checking current: Flush publishes the new pair before
		// it cancels the old one, so a pair found cancelled and then still
		// current can only have been cancelled by Close
		cancelled := c.ctx.Err() != nil
		if p.current.Load() == c {
			if cancelled && ctx.Err() == nil {
				return nil, ocache.ErrClosed
			}
			return pr, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
	}
}

// fast is the hit path: a live peer already loaded in the current pair is
// returned without blocking, allocating or touching the pair ctx (servers,
// which never flush, pay for this on every call). Anything else — a miss, an
// entry still loading or closing, a cached dial error, a closed peer, a pair
// swapped while reading — returns nil and the caller takes lookup, which
// handles those cases. Rechecking current after the read gives the same
// guarantee as lookup's post-check: the peer comes from a pair that was
// current after it was read. touch refreshes the GC deadline (Get) or not
// (Pick); hits are counted either way, as the caches themselves do. Get's
// incoming miss is counted too, so the series are the same as before.
func (p *pool) fast(id string, touch bool) peer.Peer {
	c := p.current.Load()
	v, ok := c.peekIncoming.Peek(id, touch)
	if !ok {
		if v, ok = c.peekOutgoing.Peek(id, touch); !ok {
			return nil
		}
		if touch && p.incomingMiss != nil {
			p.incomingMiss.Inc()
		}
	}
	if pr, isPeer := v.(peer.Peer); isPeer && !pr.IsClosed() && p.current.Load() == c {
		return pr
	}
	return nil
}

// discard closes pr (if not closed yet) and evicts it from source, in the
// background and outside any pool or cache lock, so the caller never waits
// past its ctx on an entry a GC TryClose holds or on a transport teardown
// (RemoveSame closes the value itself; a second Close is idempotent).
// RemoveSame never touches a replacement installed under the same id. The
// returned channel closes once the attempt is done.
func (p *pool) discard(source ocache.OCache, pr peer.Peer) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if p.closingCtx.Err() != nil {
			// pool shutdown: cache.Close evicts whatever is left
			return
		}
		_, _ = source.RemoveSame(p.closingCtx, pr.Id(), pr)
	}()
	return done
}

// evictOnClose removes the peer from the cache as soon as its underlying
// connection dies, instead of waiting for the next Get or the GC to notice.
// When the whole pool is shutting down, cache.Close already evicts every peer,
// so per-peer removal is skipped. It never outlives the peer. cache is the
// instance the peer was published into: after a Flush that is no longer the
// current one, and RemoveSame on it fails fast with ErrClosed.
func (p *pool) evictOnClose(pr peer.Peer, cache ocache.OCache, inbound bool) {
	select {
	case <-pr.CloseChan():
	case <-p.closingCtx.Done():
		return
	}
	if p.closingCtx.Err() != nil {
		// pool is shutting down; let cache.Close handle eviction
		return
	}
	if !inbound {
		// The outgoing watcher is started from inside the ocache load func,
		// before the value is published, and RemoveSame deliberately never
		// matches a still-loading entry. A connection dying that early would
		// otherwise leave the closed peer to be published with no watcher
		// left to evict it. Pick waits the load out; incoming peers are
		// published synchronously by AddPeer and need no wait.
		_, _ = cache.Pick(p.closingCtx, pr.Id())
	}
	// Remove only if the cache still holds THIS peer. A newer connection for
	// the same id may have replaced pr (incoming AddPeer re-add, or outgoing
	// redial); removing by id alone would close that live replacement.
	_, _ = cache.RemoveSame(p.closingCtx, pr.Id(), pr)
	// RemoveSame can park behind another closer; re-check so no Closed is
	// delivered once pool shutdown has begun
	if p.closingCtx.Err() != nil {
		return
	}
	p.observer.Notify(peerobserver.Event{
		Kind:    peerobserver.KindClosed,
		PeerId:  pr.Id(),
		Inbound: inbound,
	})
}

func (p *pool) Get(ctx context.Context, id string) (peer.Peer, error) {
	if pr := p.fast(id, true); pr != nil {
		return pr, nil
	}
	return p.lookup(ctx, func(ctx context.Context, c *caches) (pr peer.Peer, err error) {
		// if we have incoming connection - try to reuse it
		if pr, err = p.get(ctx, c.incoming, id); err != nil {
			// or try to get or create outgoing
			return p.get(ctx, c.outgoing, id)
		}
		return
	})
}

func (p *pool) get(ctx context.Context, source ocache.OCache, id string) (peer.Peer, error) {
	for {
		v, err := source.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		pr, err := getPeer(v)
		if err != nil {
			return nil, err
		}
		if !pr.IsClosed() {
			return pr, nil
		}
		// The entry must be gone before redialing or source.Get would return
		// the same instance again: wait (bounded by ctx) for the background
		// discard, so a teardown that blocks never runs on this path.
		select {
		case <-p.discard(source, pr):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		// with a done ctx source.Get can return the closed value again
		if err = ctx.Err(); err != nil {
			return nil, err
		}
	}
}

// Flush invalidates every pooled connection: it builds a fresh cache pair,
// publishes it and closes the replaced pair in the background, so it never
// waits on a GC TryClose or on transport teardown. From the moment it returns
// no lookup hands out a pre-flush peer (see lookup), and a dial that was in
// flight is cancelled instead of being waited out. Cached dial errors go with
// the old pair, except incompatible-version verdicts (see errObject), which
// are carried over so their backoff survives (one still loading at the swap
// is not: its verdict lands in the old pair and the fresh one redials once).
// A no-op once the pool is closed; concurrent flushes are serialized, so each
// replaced pair is closed exactly once.
func (p *pool) Flush(ctx context.Context) error {
	p.swapMu.Lock()
	if p.closed {
		p.swapMu.Unlock()
		return nil
	}
	old := p.current.Load()
	fresh := p.newCaches()
	old.outgoing.ForEach(func(v ocache.Object) (isContinue bool) {
		if eo, ok := v.(*errObject); ok && eo.keepOnFlush() {
			// cheap and non-blocking: a fresh cache has no closers
			_ = fresh.outgoing.Add(eo.id, eo)
		}
		return true
	})
	p.current.Store(fresh)
	old.cancel()
	// under swapMu like closed: Close sets closed and then waits, so no Add
	// can follow its Wait
	p.closing.Add(1)
	p.swapMu.Unlock()
	go func() {
		defer p.closing.Done()
		peers, _ := closeCaches(old)
		peers.Wait()
	}()
	return nil
}

// closeCaches tears down a pair that is no longer current. Each loaded peer is
// closed on its own goroutine first, because ocache.Close closes entries one
// at a time with no ctx and one hung teardown would hold the rest back; the
// Close passes then cancel the in-flight dials (outgoing first, so a hung
// incoming peer cannot delay that) and close the peers a second time, which
// peer.Close tolerates (the pool relies on that already, see discard). A
// RemoveSame per peer would not do: once a cache is marked closed every
// RemoveSame is refused. Known gap: a peer a GC TryClose holds past
// closeTimeout closes only when TryClose returns (ocache escalates the
// decline), never if it never returns. Returns once the caches are closed;
// the WaitGroup tracks the per-peer closes still running, and err is the
// outgoing cache's close error.
func closeCaches(c *caches) (peers *sync.WaitGroup, err error) {
	peers = &sync.WaitGroup{}
	for _, cache := range []ocache.OCache{c.outgoing, c.incoming} {
		cache.ForEach(func(v ocache.Object) (isContinue bool) {
			if pr, ok := v.(peer.Peer); ok {
				peers.Add(1)
				go func() {
					defer peers.Done()
					_ = pr.Close()
				}()
			}
			return true
		})
	}
	err = c.outgoing.Close()
	if e := c.incoming.Close(); e != nil {
		log.Warn("close incoming cache error", zap.Error(e))
	}
	return peers, err
}

func (p *pool) getIfActive(ctx context.Context, peerIds []string) peer.Peer {
	for _, peerId := range peerIds {
		if pr := p.fast(peerId, false); pr != nil {
			return pr
		}
	}
	pr, _ := p.lookup(ctx, func(ctx context.Context, c *caches) (peer.Peer, error) {
		for _, peerId := range peerIds {
			// a cached errObject (failed dial) only disqualifies this peerId,
			// not the rest of the scan
			if pr, err := p.pick(ctx, c.incoming, peerId); err == nil {
				return pr, nil
			}
			if pr, err := p.pick(ctx, c.outgoing, peerId); err == nil {
				return pr, nil
			}
		}
		return nil, errPeerNotFound
	})
	return pr
}

func (p *pool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	pr := p.getIfActive(ctx, peerIds)
	if pr != nil {
		return pr, nil
	}
	// shuffle ids for better consistency
	indexes := make([]int, len(peerIds))
	for i := range indexes {
		indexes[i] = i
	}
	rand.Shuffle(len(indexes), func(i, j int) {
		indexes[i], indexes[j] = indexes[j], indexes[i]
	})
	// connecting
	var lastErr error
	for _, idx := range indexes {
		peerId := peerIds[idx]
		if v, err := p.Get(ctx, peerId); err == nil {
			return v, nil
		} else {
			log.Debug("unable to connect", zap.String("peerId", peerId), zap.Error(err))
			lastErr = err
		}
	}
	if _, ok := lastErr.(handshake.HandshakeError); !ok {
		lastErr = net.ErrUnableToConnect
	}
	return nil, lastErr
}

// AddPeer adds an incoming peer. pr must be of a comparable type (a pointer):
// the pool evicts it by instance.
func (p *pool) AddPeer(ctx context.Context, pr peer.Peer) error {
	// bounds the retries on an entry that is neither pickable nor gone: one
	// mid-close, whose teardown may be slow (then ErrExists, as before)
	const retries = 3
	attempts := 0
	for {
		c, err := p.addIncoming(pr)
		if err != ocache.ErrExists {
			return err
		}
		// An incoming connection with this peer already exists: close and
		// remove it, then add again. This runs outside swapMu because the
		// removal can wait on a GC TryClose that holds the entry, which must
		// not stall a Flush and every AddPeer queued behind it.
		v, e := c.incoming.Pick(ctx, pr.Id())
		if e != nil {
			if err = ctx.Err(); err != nil {
				return err
			}
			if p.current.Load() != c {
				// the pair was flushed meanwhile: add to the current one
				continue
			}
			if e == ocache.ErrClosed {
				return e
			}
			// The entry was a transient one: a concurrent Get(id) creates a
			// loading entry that the incoming loader fails with ErrNotExists
			// (Pick waited that out), or the previous connection is mid-close.
			if attempts++; attempts <= retries {
				continue
			}
			return ocache.ErrExists
		}
		// The old connection's teardown (Close, then the instance-safe
		// removal that never touches a replacement) runs in the background
		// and is waited for only as long as ctx allows: a hung transport must
		// not stall the accept path.
		old, isPeer := v.(peer.Peer)
		if !isPeer {
			_, _ = c.incoming.RemoveSame(ctx, pr.Id(), v)
		} else {
			select {
			case <-p.discard(c.incoming, old):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err = ctx.Err(); err != nil {
			return err
		}
	}
}

// addIncoming adds pr to the current incoming cache and returns the pair it
// used. The read lock spans the Add, which never blocks, so a peer accepted
// after a Flush published its pair can only land in that pair, never in one
// that is about to be closed; Add therefore fails with ErrClosed only when the
// pool is closed (Close leaves the closed pair current). Returns ErrExists
// without starting a watcher.
func (p *pool) addIncoming(pr peer.Peer) (*caches, error) {
	p.swapMu.RLock()
	defer p.swapMu.RUnlock()
	c := p.current.Load()
	if err := c.incoming.Add(pr.Id(), pr); err != nil {
		return c, err
	}
	go p.evictOnClose(pr, c.incoming, true)
	return c, nil
}

func (p *pool) Pick(ctx context.Context, id string) (pr peer.Peer, err error) {
	if pr = p.fast(id, false); pr != nil {
		return pr, nil
	}
	return p.lookup(ctx, func(ctx context.Context, c *caches) (pr peer.Peer, err error) {
		// check if connection with peer exist without dial
		if pr, err = p.pick(ctx, c.incoming, id); err != nil {
			return p.pick(ctx, c.outgoing, id)
		}
		return
	})
}

func (p *pool) pick(ctx context.Context, source ocache.OCache, id string) (peer.Peer, error) {
	v, err := source.Pick(ctx, id)
	if err != nil {
		return nil, err
	}
	// the cache can hold an *errObject for a failed dial: resolve through
	// getPeer like get() does instead of panicking on the type assertion
	pr, err := getPeer(v)
	if err != nil {
		return nil, err
	}
	if !pr.IsClosed() {
		return pr, nil
	}
	p.discard(source, pr)
	return nil, errPeerNotFound
}

func (p *pool) ProvideStat() any {
	peerStats := make([]*peer.Stat, 0)
	c := p.current.Load()
	c.outgoing.ForEach(func(v ocache.Object) (isContinue bool) {
		if p, ok := v.(peer.StatProvider); ok {
			peerStats = append(peerStats, p.ProvideStat())
		}
		return true
	})
	c.incoming.ForEach(func(v ocache.Object) (isContinue bool) {
		if p, ok := v.(peer.StatProvider); ok {
			peerStats = append(peerStats, p.ProvideStat())
		}
		return true
	})
	return &poolStats{PeerStats: peerStats}
}

func (p *pool) StatId() string {
	return CName
}

func (p *pool) StatType() string {
	return CName
}

var errPeerNotFound = fmt.Errorf("failed to pick connection with peer: peer not found")

func getPeer(val ocache.Object) (pr peer.Peer, err error) {
	switch v := val.(type) {
	case peer.Peer:
		pr = v
	case *errObject:
		err = v.Error()
	default:
		err = fmt.Errorf("unknown peer type: %T", val)
	}
	return
}
