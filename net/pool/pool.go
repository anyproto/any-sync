//go:generate mockgen -destination mock_pool/mock_pool.go github.com/anyproto/any-sync/net/pool Pool,Service
package pool

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"

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
	// AddPeer adds incoming peer to the pool. The pool keys per-instance state
	// by the peer, so peer.Peer implementations must be comparable (pointers)
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

type pool struct {
	outgoing      ocache.OCache
	incoming      ocache.OCache
	statService   debugstat.StatService
	closingCtx    context.Context
	closingCancel context.CancelFunc
	// observer is bound once at Init (peerobserver.CName) and never changes
	// afterwards, so it is read without locking; its zero value is a no-op
	observer peerobserver.Notifier

	// gen is the pool generation. Flush bumps it before anything else, and
	// every lookup treats a peer stamped with an older generation as closed.
	// That keeps a flush effective for peers its removal pass cannot see: one
	// a GC TryClose holds in closing and then restores because the peer
	// declined, or one an outgoing dial started before the flush publishes
	// after it.
	gen atomic.Uint64
	// stamps maps each pooled peer instance to the generation it belongs to:
	// read before dialing for outgoing peers, at AddPeer for incoming ones.
	// Kept pool-side so the peer.Peer interface stays untouched; it requires
	// comparable (pointer) peer.Peer implementations. It also holds peers the
	// cache does not list (loading, or held by a closer), so Flush can close
	// every stale one. The peer's evictOnClose watcher drops the entry; a peer
	// without a stamp counts as current (it is either closed or was never
	// added through the pool).
	stampsMu sync.Mutex
	stamps   map[peer.Peer]*peerStamp
}

type peerStamp struct {
	gen    uint64
	source ocache.OCache
	// discarded is set once the async close of a rejected peer is queued,
	// so repeated lookups and flushes queue it only once; closed when done
	discarded chan struct{}
}

func (p *pool) stamp(pr peer.Peer, gen uint64, source ocache.OCache) {
	p.stampsMu.Lock()
	defer p.stampsMu.Unlock()
	if p.stamps == nil {
		p.stamps = map[peer.Peer]*peerStamp{}
	}
	p.stamps[pr] = &peerStamp{gen: gen, source: source}
}

// stampCurrent stamps pr with the current generation. Reading the
// generation under stampsMu orders it against Flush's bump-and-snapshot:
// either the stamp is in Flush's snapshot and pr gets discarded, or it
// carries the new generation.
func (p *pool) stampCurrent(pr peer.Peer, source ocache.OCache) {
	p.stampsMu.Lock()
	defer p.stampsMu.Unlock()
	if p.stamps == nil {
		p.stamps = map[peer.Peer]*peerStamp{}
	}
	p.stamps[pr] = &peerStamp{gen: p.gen.Load(), source: source}
}

func (p *pool) unstamp(pr peer.Peer) {
	p.stampsMu.Lock()
	defer p.stampsMu.Unlock()
	delete(p.stamps, pr)
}

// isStale reports whether pr belongs to a generation before the latest Flush
func (p *pool) isStale(pr peer.Peer) bool {
	cur := p.gen.Load()
	if cur == 0 {
		// never flushed (servers never flush): skip the pool-wide lock
		return false
	}
	p.stampsMu.Lock()
	defer p.stampsMu.Unlock()
	st, ok := p.stamps[pr]
	return ok && st.gen < cur
}

// usable reports whether pr may be handed out. A closed or stale peer is
// discarded on the way.
func (p *pool) usable(source ocache.OCache, pr peer.Peer) bool {
	// stale first: the stamp is dropped only after the peer closed, so a
	// stale peer whose stamp vanishes between the two checks is still seen
	// as closed by the second
	if !p.isStale(pr) && !pr.IsClosed() {
		return true
	}
	p.discard(source, pr)
	return false
}

// discard closes pr and evicts exactly this instance from source, in the
// background and outside any pool or cache lock, so the caller never waits on
// a GC TryClose holding the entry or on the transport teardown. Closing the
// peer wakes its evictOnClose watcher, which delivers the Closed event, so
// Connected/Closed stay paired for rejected peers. RemoveSame never touches a
// replacement installed under the same id. The returned channel closes once
// the close and the removal attempt are done.
func (p *pool) discard(source ocache.OCache, pr peer.Peer) <-chan struct{} {
	done := make(chan struct{})
	p.stampsMu.Lock()
	if st, ok := p.stamps[pr]; ok {
		if st.discarded != nil {
			p.stampsMu.Unlock()
			return st.discarded
		}
		st.discarded = done
	}
	p.stampsMu.Unlock()
	go func() {
		defer close(done)
		_ = pr.Close()
		if p.closingCtx.Err() != nil {
			// pool shutdown: cache.Close evicts whatever is left
			return
		}
		_, _ = source.RemoveSame(p.closingCtx, pr.Id(), pr)
	}()
	return done
}

// discardErrObject drops a cached dial error
func (p *pool) discardErrObject(ctx context.Context, source ocache.OCache, eo *errObject) {
	// cheap and non-blocking: errObject's Close and TryClose never block
	_, _ = source.RemoveSame(ctx, eo.id, eo)
}

func (p *pool) Name() (name string) {
	return CName
}

// evictOnClose removes the peer from the cache as soon as its underlying
// connection dies, instead of waiting for the next Get or the GC to notice.
// When the whole pool is shutting down, cache.Close already evicts every peer,
// so per-peer removal is skipped. It never outlives the peer.
func (p *pool) evictOnClose(pr peer.Peer, cache ocache.OCache, inbound bool) {
	defer p.unstamp(pr)
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

func (p *pool) Get(ctx context.Context, id string) (pr peer.Peer, err error) {
	// if we have incoming connection - try to reuse it
	if pr, err = p.get(ctx, p.incoming, id); err != nil {
		// or try to get or create outgoing
		return p.get(ctx, p.outgoing, id)
	}
	return
}

func (p *pool) get(ctx context.Context, source ocache.OCache, id string) (peer.Peer, error) {
	v, err := source.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if eo, ok := v.(*errObject); ok && eo.flushed(p.gen.Load()) {
		// a dial error cached before the latest Flush: drop it and redial
		p.discardErrObject(ctx, source, eo)
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		return p.Get(ctx, id)
	}
	pr, err := getPeer(v)
	if err != nil {
		return nil, err
	}
	if p.usable(source, pr) {
		return pr, nil
	}
	// The entry must be gone before redialing or source.Get would return the
	// same instance again: wait (bounded by ctx) for the background discard,
	// so the close never runs on this path.
	select {
	case <-p.discard(source, pr):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// The discard's RemoveSame misses an entry published after it ran (a
	// late dial). pr is closed by now, so taking it out here costs only an
	// idempotent Close; instance-safe, so a replacement installed meanwhile
	// survives.
	_, _ = source.RemoveSame(ctx, id, pr)
	// with a done ctx source.Get can return the loaded stale value again
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return p.Get(ctx, id)
}

// Flush invalidates every pooled connection. It bumps the pool generation
// first: from then on no lookup returns a peer created before the call,
// whatever state the cache entry is in (see gen). It then queues the close of
// every stale peer, including ones the cache does not list (still loading, or
// held by a GC TryClose), and never waits on a GC TryClose or on transport
// teardown. Peers of the new generation are left alone. Cached dial errors
// are dropped as well, except incompatible-version ones (see errObject), so
// the first lookup after a flush redials at once.
func (p *pool) Flush(ctx context.Context) error {
	type stalePeer struct {
		pr     peer.Peer
		source ocache.OCache
	}
	var stale []stalePeer
	p.stampsMu.Lock()
	// bumped under stampsMu, see stampCurrent
	cur := p.gen.Add(1)
	for pr, st := range p.stamps {
		if st.gen < cur && st.discarded == nil {
			stale = append(stale, stalePeer{pr: pr, source: st.source})
		}
	}
	p.stampsMu.Unlock()
	for _, sp := range stale {
		p.discard(sp.source, sp.pr)
	}
	// Reserved for cached dial errors that a flush should drop: today the
	// loader caches only incompatible-version verdicts, which are kept (see
	// errObject.flushed), so this pass and the matching branches in get and
	// pick find nothing.
	for _, source := range []ocache.OCache{p.incoming, p.outgoing} {
		var errObjects []*errObject
		source.ForEach(func(v ocache.Object) (isContinue bool) {
			if eo, ok := v.(*errObject); ok && eo.flushed(cur) {
				errObjects = append(errObjects, eo)
			}
			return true
		})
		// one published after this scan carries an older generation and is
		// dropped by get/pick instead
		for _, eo := range errObjects {
			p.discardErrObject(ctx, source, eo)
		}
	}
	return nil
}

func (p *pool) getIfActive(ctx context.Context, peerIds []string) peer.Peer {
	for _, peerId := range peerIds {
		// a cached errObject (failed dial) only disqualifies this peerId,
		// not the rest of the scan
		if v, err := p.incoming.Pick(ctx, peerId); err == nil {
			if pr, err := getPeer(v); err == nil && p.usable(p.incoming, pr) {
				return pr
			}
		}
		if v, err := p.outgoing.Pick(ctx, peerId); err == nil {
			if pr, err := getPeer(v); err == nil && p.usable(p.outgoing, pr) {
				return pr
			}
		}
	}
	return nil
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
// the pool keys its per-instance bookkeeping by it.
func (p *pool) AddPeer(ctx context.Context, pr peer.Peer) (err error) {
	// stamped before it becomes visible, so no lookup sees it unstamped
	p.stampCurrent(pr, p.incoming)
	err = p.incoming.Add(pr.Id(), pr)
	if err == ocache.ErrExists {
		// in case when an incoming connection with a peer already exists, we close and remove an existing connection
		if v, e := p.incoming.Pick(ctx, pr.Id()); e == nil {
			_ = v.Close()
			// instance-safe: never removes a replacement that took the id
			// in the meantime
			_, _ = p.incoming.RemoveSame(ctx, pr.Id(), v)
			err = p.incoming.Add(pr.Id(), pr)
		}
	}
	if err == nil {
		go p.evictOnClose(pr, p.incoming, true)
	} else {
		p.unstamp(pr)
	}
	return err
}

func (p *pool) Pick(ctx context.Context, id string) (pr peer.Peer, err error) {
	// check if connection with peer exist without dial
	if pr, err = p.pick(ctx, p.incoming, id); err != nil {
		return p.pick(ctx, p.outgoing, id)
	}
	return
}

func (p *pool) pick(ctx context.Context, source ocache.OCache, id string) (peer.Peer, error) {
	v, err := source.Pick(ctx, id)
	if err != nil {
		return nil, err
	}
	// the cache can hold an *errObject for a failed dial: resolve through
	// getPeer like get() does instead of panicking on the type assertion
	if eo, ok := v.(*errObject); ok && eo.flushed(p.gen.Load()) {
		// a dial error cached before the latest Flush no longer counts
		p.discardErrObject(ctx, source, eo)
		return nil, errPeerNotFound
	}
	pr, err := getPeer(v)
	if err != nil {
		return nil, err
	}
	if p.usable(source, pr) {
		return pr, nil
	}
	return nil, errPeerNotFound
}

func (p *pool) ProvideStat() any {
	peerStats := make([]*peer.Stat, 0)
	p.outgoing.ForEach(func(v ocache.Object) (isContinue bool) {
		if p, ok := v.(peer.StatProvider); ok {
			peerStats = append(peerStats, p.ProvideStat())
		}
		return true
	})
	p.incoming.ForEach(func(v ocache.Object) (isContinue bool) {
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
