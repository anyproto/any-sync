//go:generate mockgen -destination mock_pool/mock_pool.go github.com/anyproto/any-sync/net/pool Pool,Service
package pool

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
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

// The pool keeps one immutable pair of caches (incoming, outgoing) behind an
// atomic pointer. Invariants:
//   - a lookup reads one pair and re-checks after the read that it is still
//     current, so no result comes from a replaced pair (lookup, fast);
//   - Flush swaps the pair under swapMu, cancels the old one (cutting every
//     lookup on it short) and closes it in the background; AddPeer adds under
//     the read side of swapMu, so a peer never lands in a replaced pair;
//   - the Flush that replaced a pair walks it once afterwards: it reports the
//     Closed events and marks the peers, and a peer's watcher reports only a
//     peer that is not marked, so every instance is reported exactly once;
//   - Close marks the pool closed under swapMu, after which Flush is a no-op
//     and lookups fail with ocache.ErrClosed.

// Pool creates and caches outgoing connection
type Pool interface {
	// Get lookups to peer in existing connections or creates and outgoing new one
	Get(ctx context.Context, id string) (peer.Peer, error)
	// GetOneOf searches at least one existing connection in outgoing or creates a new one from a randomly selected id from given list
	GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error)
	// AddPeer adds incoming peer to the pool. The pool tracks peers by
	// instance; an implementation whose type is not comparable (not a
	// pointer) is tracked by id instead, which is only weaker when two
	// connections for one id overlap
	AddPeer(ctx context.Context, p peer.Peer) (err error)
	// Pick checks if a connection with the peer exists, without dialing.
	// For a peer whose last dial failed it returns the cached dial error.
	Pick(ctx context.Context, id string) (pr peer.Peer, err error)
	// Flush invalidates every pooled connection and every dial in flight: it
	// swaps in an empty pool, reports Closed for each pooled peer before it
	// returns and tears the old connections down in the background. Later
	// lookups redial. Callers should coalesce their triggers, since each
	// Flush cancels the dials of the one before.
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
	// reported holds the peers (see peerKey) whose Closed event Flush
	// delivered itself, so their watchers do not report them a second time
	// (see Flush); written once, by Flush, under reportedMu
	reportedMu sync.Mutex
	reported   map[any]struct{}
}

// peerKey identifies a pooled peer instance in a map: the peer itself when it
// is comparable (a pointer, as every implementation in this module is), else
// its id and direction, which within one pair name a single entry at a time.
// Never panics on a non-comparable implementation: checked on the value, since
// a type with an interface field is comparable while the value may not be.
func peerKey(pr peer.Peer, inbound bool) any {
	if reflect.ValueOf(pr).Comparable() {
		return pr
	}
	return struct {
		id      string
		inbound bool
	}{pr.Id(), inbound}
}

// bind derives from ctx a context that also ends when the pair stops being
// current; release frees it and must be called once the work is done
func (c *caches) bind(ctx context.Context) (bound context.Context, release func()) {
	bound, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(c.ctx, cancel)
	return bound, func() {
		stop()
		cancel()
	}
}

// cache returns the incoming or the outgoing cache of the pair
func (c *caches) cache(inbound bool) ocache.OCache {
	if inbound {
		return c.incoming
	}
	return c.outgoing
}

// reportedByFlush reports whether Flush delivered this peer's Closed event
func (c *caches) reportedByFlush(pr peer.Peer, inbound bool) bool {
	c.reportedMu.Lock()
	defer c.reportedMu.Unlock()
	_, ok := c.reported[peerKey(pr, inbound)]
	return ok
}

// fastMetrics are the counters the pool bumps itself where it reads a cache
// through Peek, which counts nothing (nil without a registry): a Get or Pick
// counts exactly once per cache, whichever path serves it, the series being
// those a Get through both caches produced before
type fastMetrics struct {
	incomingHit, incomingMiss, outgoingHit prometheus.Counter
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
	metrics      *fastMetrics

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
			lctx, release := c.bind(ctx)
			defer release()
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
// (Pick). Metrics are counted only for a peer actually returned, and as the
// caches would have counted them (a hit; for Get also the incoming miss
// before an outgoing hit), so a fall-through to lookup counts nothing twice.
// missed is the pair whose caches both held nothing for id while it stayed
// current (nil otherwise): Get skips its own incoming probe once on that very
// pair (the miss is counted here, as a Get would have).
func (p *pool) fast(id string, touch bool) (pr peer.Peer, missed *caches) {
	c := p.current.Load()
	v, in := c.peekIncoming.Peek(id, touch)
	inbound := in == ocache.PeekHit
	if !inbound {
		var out ocache.PeekState
		if v, out = c.peekOutgoing.Peek(id, touch); out != ocache.PeekHit {
			// a busy entry (loading, or a close that may yet be declined)
			// is not a miss: the lookup waits for it
			if in != ocache.PeekMiss || out != ocache.PeekMiss || p.current.Load() != c {
				return nil, nil
			}
			if touch && p.metrics != nil {
				p.metrics.incomingMiss.Inc()
			}
			return nil, c
		}
	}
	pr, isPeer := v.(peer.Peer)
	if !isPeer || pr.IsClosed() || p.current.Load() != c {
		return nil, nil
	}
	if m := p.metrics; m != nil {
		switch {
		case inbound:
			m.incomingHit.Inc()
		default:
			if touch {
				m.incomingMiss.Inc()
			}
			m.outgoingHit.Inc()
		}
	}
	return pr, nil
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
// connection dies, instead of waiting for the next Get or the GC to notice,
// and reports the Closed event unless Flush already did. When the whole pool
// is shutting down, cache.Close already evicts every peer, so per-peer removal
// is skipped. It never outlives the peer. c is the pair the peer was
// published into: after a Flush that is no longer the current one, and
// RemoveSame on it fails fast with ErrClosed.
func (p *pool) evictOnClose(pr peer.Peer, c *caches, inbound bool) {
	cache := c.cache(inbound)
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
	// delivered once pool shutdown has begun. Checked after the removal: Flush
	// marks the peers it saw and reports under reportedMu, and the removal
	// and Flush's snapshot are ordered by the cache lock, so a peer Flush saw
	// is marked by the time this runs and one it did not see is reported here
	if p.closingCtx.Err() != nil || c.reportedByFlush(pr, inbound) {
		return
	}
	p.observer.Notify(peerobserver.Event{
		Kind:    peerobserver.KindClosed,
		PeerId:  pr.Id(),
		Inbound: inbound,
	})
}

func (p *pool) Get(ctx context.Context, id string) (peer.Peer, error) {
	pr, missed := p.fast(id, true)
	if pr != nil {
		return pr, nil
	}
	return p.lookup(ctx, func(ctx context.Context, c *caches) (pr peer.Peer, err error) {
		for {
			// if we have incoming connection - try to reuse it (the fast path
			// just did when it found this very pair empty)
			if c == missed {
				missed = nil
				err = ocache.ErrNotExists
			} else {
				pr, err = p.getIncoming(ctx, c, id)
			}
			switch {
			case err == nil:
				return pr, nil
			case err == ocache.ErrNotExists:
				// or try to get or create outgoing. Only on a real miss: an
				// ErrClosed (the pair was replaced; lookup retries on the
				// current one) or a ctx error must not start a dial, the
				// former into the old pair with a ctx the swap has not
				// cancelled yet (that runs through an AfterFunc,
				// asynchronously), the latter with a dead one
				if pr, err = p.get(ctx, c.outgoing, id); err != errRedial {
					return pr, err
				}
			case err != errRedial:
				return nil, err
			}
			// A closed peer was evicted: start over from incoming, where a
			// live connection may have arrived meanwhile, before dialing. Not
			// with a ctx that ended meanwhile (the eviction's select picks at
			// random when both are ready) and not during shutdown, when
			// nothing is evicted any more and the closed peer would be found
			// again and again. A replaced pair is caught by the incoming probe
			// and by the loader, which never dials for one.
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			if p.closingCtx.Err() != nil {
				return nil, ocache.ErrClosed
			}
		}
	})
}

// errRedial is the verdict after a closed peer was evicted: look again,
// starting from the incoming cache
var errRedial = errors.New("closed peer evicted")

// getIncoming reads the incoming cache without loading: its entries come from
// AddPeer only, so a load would just insert a failing entry for the time of
// the probe (and make every AddPeer for that id trip over it). Counted like a
// Get on the cache would be: a hit, or a miss before the outgoing lookup.
func (p *pool) getIncoming(ctx context.Context, c *caches, id string) (peer.Peer, error) {
	v, state := c.peekIncoming.Peek(id, true)
	// An entry another closer holds may be a GC TryClose the live peer
	// declines: wait it out (bounded by ctx and the pair, like a Get on the
	// cache would) and look again before dialing a second connection to the
	// peer. Bounded: a closer that keeps coming back is treated as a miss.
	for attempt := 0; state == ocache.PeekBusy && attempt < 3; attempt++ {
		bctx, release := c.bind(ctx)
		err := c.peekIncoming.WaitClosing(bctx, id)
		release()
		if err != nil {
			if c.ctx.Err() != nil {
				return nil, ocache.ErrClosed
			}
			return nil, err
		}
		v, state = c.peekIncoming.Peek(id, true)
	}
	if state != ocache.PeekHit && c.ctx.Err() != nil {
		// a replaced or closing pair: no verdict about the peer, and not a
		// miss (a Get on the cache would have failed with ErrClosed uncounted)
		return nil, ocache.ErrClosed
	}
	if m := p.metrics; m != nil {
		if state == ocache.PeekHit {
			m.incomingHit.Inc()
		} else {
			m.incomingMiss.Inc()
		}
	}
	if state != ocache.PeekHit {
		return nil, ocache.ErrNotExists
	}
	return p.live(ctx, c.incoming, v)
}

func (p *pool) get(ctx context.Context, source ocache.OCache, id string) (peer.Peer, error) {
	v, err := source.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return p.live(ctx, source, v)
}

// live resolves a cached value to an open peer. A closed one is evicted first:
// the entry must be gone before looking again or the cache would return the
// same instance, so this waits (bounded by ctx) for the background discard,
// and a teardown that blocks never runs on this path.
func (p *pool) live(ctx context.Context, source ocache.OCache, v ocache.Object) (peer.Peer, error) {
	pr, err := getPeer(v)
	if err != nil {
		return nil, err
	}
	if !pr.IsClosed() {
		return pr, nil
	}
	select {
	case <-p.discard(source, pr):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return nil, errRedial
}

// Flush invalidates every pooled connection: it builds a fresh pair, carries
// the cached dial verdicts over (incompatible-version ones, whose backoff must
// survive; one still loading at the swap is lost and redialed once), publishes
// it, cancels the old pair, which cuts every dial in flight short, and closes
// it in the background, so it never waits on a GC TryClose or on transport
// teardown. The Closed event of every peer found pooled is delivered on the
// calling goroutine before it returns (observers must not be blocked by a lock
// the caller holds): for the caller's own later calls a Closed(X) precedes the
// Connected(X) of the redial; a concurrent Get or Accept can still produce its
// Connected first. A peer a GC TryClose holds at that moment is reported by
// its watcher once it closes; once pool shutdown has begun the events are
// suppressed like the watchers'. A no-op once the pool is closed; concurrent
// flushes are safe (see the invariants at the top of the file).
func (p *pool) Flush(ctx context.Context) error {
	p.swapMu.Lock()
	if p.closed {
		p.swapMu.Unlock()
		return nil
	}
	old := p.current.Load()
	fresh := p.newCaches()
	// the verdicts must be in the fresh pair before it is published, so this
	// one read of the old outgoing cache happens before the swap
	old.outgoing.ForEach(func(v ocache.Object) (isContinue bool) {
		if eo, ok := v.(*errObject); ok {
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
	// The walk over the old pair comes after the swap: addIncoming adds under
	// the read lock, so every incoming peer of the old pair is visible now,
	// and only this Flush (the one that replaced the pair) walks it, so the
	// marks are set exactly once. It serves the marking, the Closed events
	// and the parallel pre-close alike.
	peers := old.snapshot(true)
	go func() {
		defer p.closing.Done()
		_ = closeCaches(old, peers)
	}()
	for _, f := range peers {
		if p.closingCtx.Err() != nil {
			// a Close that began meanwhile: Closed is suppressed from here on
			break
		}
		p.observer.Notify(peerobserver.Event{
			Kind:    peerobserver.KindClosed,
			PeerId:  f.pr.Id(),
			Inbound: f.inbound,
		})
	}
	return nil
}

type snapshotPeer struct {
	pr      peer.Peer
	inbound bool
}

// closePeer is the pre-close of one peer (named: the tests tell this close
// from the cache's own pass by it)
func closePeer(pr peer.Peer) {
	_ = pr.Close()
}

// maxPeerClosers caps how many peers closeCaches closes at once
const maxPeerClosers = 256

// snapshot lists the loaded peers of both caches (not the ones another closer
// holds). With mark, every peer found is recorded as reported by Flush (see
// evictOnClose) under reportedMu, which stays held across the walk so the
// marks are complete by the time a watcher reads them.
func (c *caches) snapshot(mark bool) (peers []snapshotPeer) {
	if mark {
		c.reportedMu.Lock()
		defer c.reportedMu.Unlock()
		c.reported = map[any]struct{}{}
	}
	for _, inbound := range []bool{true, false} {
		c.cache(inbound).ForEach(func(v ocache.Object) (isContinue bool) {
			if pr, ok := v.(peer.Peer); ok {
				if mark {
					c.reported[peerKey(pr, inbound)] = struct{}{}
				}
				peers = append(peers, snapshotPeer{pr: pr, inbound: inbound})
			}
			return true
		})
	}
	return peers
}

// closeCaches tears down a pair that is no longer current and returns once
// its caches and the given peers are closed. The peers are closed in
// parallel first (up to maxPeerClosers at a time), because ocache.Close
// closes entries one at a time with no ctx and one hung teardown would hold
// the rest back; the two caches are then
// closed concurrently (each Close cancels its in-flight loads and closes its
// peers a second time, which peer.Close tolerates; the pool relies on that
// already, see discard), so a hung peer in one cache never delays the other.
// A RemoveSame per peer would not do: once a cache is marked closed every
// RemoveSame is refused. Known gap: a peer a GC TryClose holds past
// closeTimeout closes only when TryClose returns (ocache escalates the
// decline), never if it never returns. err is the outgoing cache's close
// error.
func closeCaches(c *caches, peers []snapshotPeer) (err error) {
	var wg sync.WaitGroup
	// The pre-close runs at most maxPeerClosers peers at a time: a server
	// pool holds tens of thousands, and one goroutine each would be a burst
	// that outlives a Close which gave up. The dispatch runs on its own
	// goroutine so the cache closes below (which cancel the in-flight dials)
	// are not held back behind hung peers waiting for a slot; its own wg
	// count keeps Wait from returning before every peer has been dispatched.
	wg.Add(1)
	go func() {
		defer wg.Done()
		slots := make(chan struct{}, maxPeerClosers)
		for _, sp := range peers {
			slots <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() {
					<-slots
					wg.Done()
				}()
				closePeer(sp.pr)
			}()
		}
	}()
	incomingClosed := make(chan error, 1)
	go func() { incomingClosed <- c.incoming.Close() }()
	err = c.outgoing.Close()
	if e := <-incomingClosed; e != nil {
		log.Warn("close incoming cache error", zap.Error(e))
	}
	wg.Wait()
	return err
}

func (p *pool) getIfActive(ctx context.Context, peerIds []string) peer.Peer {
	for _, peerId := range peerIds {
		if pr, _ := p.fast(peerId, false); pr != nil {
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

// AddPeer adds an incoming peer. The pool evicts it by instance (see peerKey
// and ocache.RemoveSame for non-comparable implementations).
func (p *pool) AddPeer(ctx context.Context, pr peer.Peer) error {
	// Bounds the passes over an entry for the same id that is still there
	// after this call dealt with it: one another closer holds (its close is
	// waited for below), or one another AddPeer keeps replacing. Then
	// ErrExists, as before. A swap does not count: the add simply moves to
	// the current pair, as many times as flushes come (in practice the caller
	// coalesces its flushes).
	const retries = 3
	attempts := 0
	for {
		if p.closingCtx.Err() != nil {
			// shutting down: nothing is evicted any more (discard is a no-op),
			// so there is nothing to retry towards
			return ocache.ErrClosed
		}
		c, err := p.addIncoming(pr)
		if err != ocache.ErrExists {
			return err
		}
		// An incoming connection with this peer already exists: close and
		// remove it, then add again. This runs outside swapMu because the
		// removal can wait on a GC TryClose that holds the entry, which must
		// not stall a Flush and every AddPeer queued behind it.
		v, e := c.incoming.Pick(ctx, pr.Id())
		if p.current.Load() != c {
			// the pair was flushed meanwhile: whatever it holds is the
			// flush's to tear down; add to the current pair instead
			continue
		}
		if e == ocache.ErrClosed {
			return e
		}
		if attempts++; attempts > retries {
			return ocache.ErrExists
		}
		if e != nil {
			if err = ctx.Err(); err != nil {
				return err
			}
			// The entry is mid-close (nothing else is ever in this cache
			// unloaded: it is filled by AddPeer alone, never by a load). Pick
			// does not wait for a close, so wait here: a remote that
			// reconnects while its old connection is still being torn down
			// must get in once that is done, not be refused within
			// microseconds. Bounded by ctx and by the pair staying current.
			if err = p.waitClosing(ctx, c, pr.Id()); err != nil && c.ctx.Err() == nil {
				return err
			}
			continue
		}
		// The old connection's teardown (Close, then the instance-safe
		// removal that never touches a replacement) runs in the background
		// and is waited for only as long as ctx allows and the pair stays
		// current: a hung transport must not stall the accept path. The
		// incoming cache holds peers only.
		select {
		case <-p.discard(c.incoming, v.(peer.Peer)):
		case <-c.ctx.Done():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// waitClosing waits for the incoming entry for id of pair c to finish
// closing, bounded by ctx and by c staying current
func (p *pool) waitClosing(ctx context.Context, c *caches, id string) error {
	lctx, release := c.bind(ctx)
	defer release()
	return c.peekIncoming.WaitClosing(lctx, id)
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
	go p.evictOnClose(pr, c, true)
	return c, nil
}

func (p *pool) Pick(ctx context.Context, id string) (pr peer.Peer, err error) {
	if pr, _ = p.fast(id, false); pr != nil {
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
	// a closed peer is on its way out: its watcher evicts it
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
