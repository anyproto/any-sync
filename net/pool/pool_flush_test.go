package pool

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	atomic2 "go.uber.org/atomic"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peerobserver"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
)

// ctlPeer is a test peer whose TryClose, Close and IsClosed can be paused or
// made to decline, to drive the races between Flush, the ocache GC and the
// lookups
type ctlPeer struct {
	*testPeer
	tryClose     func() (bool, error)
	closeHook    func(call int32)
	isClosedHook func(call int32)
	// idHook pauses inside addIncoming's Add, between the snapshot and the
	// insert (Id is the first thing the pool asks a new incoming peer)
	idHook       func(call int32)
	closeCalls   atomic.Int32
	isClosedCall atomic.Int32
	idCall       atomic.Int32
}

func newCtlPeer(id string) *ctlPeer {
	return &ctlPeer{testPeer: newTestPeer(id)}
}

func (c *ctlPeer) TryClose(objectTTL time.Duration) (bool, error) {
	if c.tryClose != nil {
		return c.tryClose()
	}
	return c.testPeer.TryClose(objectTTL)
}

func (c *ctlPeer) Close() error {
	call := c.closeCalls.Add(1)
	if c.closeHook != nil {
		c.closeHook(call)
	}
	return c.testPeer.Close()
}

func (c *ctlPeer) IsClosed() bool {
	call := c.isClosedCall.Add(1)
	if c.isClosedHook != nil {
		c.isClosedHook(call)
	}
	return c.testPeer.IsClosed()
}

func (c *ctlPeer) Id() string {
	call := c.idCall.Add(1)
	if c.idHook != nil {
		c.idHook(call)
	}
	return c.testPeer.Id()
}

var _ peer.Peer = (*ctlPeer)(nil)

// newRelease returns a gate channel and an idempotent func that opens it
func newRelease() (chan struct{}, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return ch, func() { once.Do(func() { close(ch) }) }
}

// pairTracker wraps the pool's cache factory to number every pair it builds
type pairTracker struct {
	mu    sync.Mutex
	seq   map[*caches]int64
	pairs []*caches
}

func trackPairs(p *pool) *pairTracker {
	tr := &pairTracker{seq: map[*caches]int64{}}
	cur := p.current.Load()
	tr.seq[cur] = 0
	tr.pairs = append(tr.pairs, cur)
	orig := p.newCaches
	p.newCaches = func() *caches {
		c := orig()
		tr.mu.Lock()
		defer tr.mu.Unlock()
		tr.seq[c] = int64(len(tr.pairs))
		tr.pairs = append(tr.pairs, c)
		return c
	}
	return tr
}

func (tr *pairTracker) seqOf(c *caches) int64 {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.seq[c]
}

func (tr *pairTracker) created() int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return len(tr.pairs)
}

// all returns a copy of the pairs built so far, oldest first
func (tr *pairTracker) all() []*caches {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return append([]*caches(nil), tr.pairs...)
}

// isClosed reports whether both caches of the pair are closed, without
// touching them: Pick fails with ErrClosed on a closed cache and with
// ErrNotExists on an open one
func (c *caches) isClosed() bool {
	_, inErr := c.incoming.Pick(ctx, "\x00probe")
	_, outErr := c.outgoing.Pick(ctx, "\x00probe")
	return inErr == ocache.ErrClosed && outErr == ocache.ErrClosed
}

func inCurrent(p *pool, pr peer.Peer) bool {
	v, err := p.current.Load().incoming.Pick(ctx, pr.Id())
	return err == nil && v == ocache.Object(pr)
}

// startFlusher flushes the pool every period on a background goroutine until
// stop is called; stop also waits for the goroutine. flushes counts the
// completed flushes.
func startFlusher(t *testing.T, fx *fixture, period time.Duration) (flushes *atomic.Int64, stop func()) {
	flushes = &atomic.Int64{}
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-quit:
				return
			default:
			}
			assert.NoError(t, fx.Flush(ctx))
			flushes.Add(1)
			time.Sleep(period)
		}
	}()
	var once sync.Once
	return flushes, func() {
		once.Do(func() {
			close(quit)
			<-done
		})
	}
}

// fromClosePrepass reports whether the caller runs on one of closeCaches's
// per-peer goroutines (the parallel teardown) rather than on the cache's own
// close pass, which closes the same peer a second time on the closing
// goroutine; test seam for "Close waits for the parallel closes"
func fromClosePrepass() bool {
	buf := make([]byte, 1<<14)
	n := runtime.Stack(buf, false)
	return strings.Contains(string(buf[:n]), "pool.closeCaches.func")
}

// hookCtx is a pair ctx whose Err can be paused: a seam between lookup's f
// returning and its read of the pair state
type hookCtx struct {
	context.Context
	onErr func()
}

func (c *hookCtx) Err() error {
	if c.onErr != nil {
		c.onErr()
	}
	return c.Context.Err()
}

func TestPool_FlushSwap(t *testing.T) {
	t.Run("peer restored by a declined TryClose across flush is never returned", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		old := newCtlPeer("p1")
		inTryClose := make(chan struct{})
		releaseTryClose, doReleaseTryClose := newRelease()
		// released on every exit, so a failing assertion fails instead of
		// hanging fx.Finish behind the paused hook
		defer doReleaseTryClose()
		old.tryClose = func() (bool, error) {
			close(inTryClose)
			<-releaseTryClose
			return false, nil // a recently used sub conn keeps the peer alive
		}
		fresh := newTestPeer("p1")
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				return old, nil
			}
			return fresh, nil
		}
		pr, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		require.Equal(t, peer.Peer(old), pr)
		oldPair := p.current.Load()

		// the GC path: the entry is held in closing while TryClose runs
		gcDone := make(chan struct{})
		go func() {
			defer close(gcDone)
			_, _ = oldPair.outgoing.TryRemove("p1")
		}()
		<-inTryClose

		// the swap does not wait on the closer holding the entry
		flushed := make(chan error, 1)
		go func() { flushed <- fx.Flush(ctx) }()
		select {
		case err = <-flushed:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("Flush waited on TryClose")
		}

		doReleaseTryClose()
		<-gcDone

		// the decline lands in a closed cache: escalated to a Close there,
		// and the current pair never saw the peer
		_, err = fx.Pick(ctx, "p1")
		require.Error(t, err)
		require.Nil(t, p.getIfActive(ctx, []string{"p1"}))
		pr, err = fx.Get(ctx, "p1")
		require.NoError(t, err)
		assert.Equal(t, peer.Peer(fresh), pr)
		pr, err = fx.GetOneOf(ctx, []string{"p1"})
		require.NoError(t, err)
		assert.Equal(t, peer.Peer(fresh), pr)
		require.Eventually(t, old.IsClosed, time.Second, 10*time.Millisecond)
		require.Eventually(t, func() bool { return oldPair.outgoing.Len() == 0 }, time.Second, 10*time.Millisecond)
	})
	t.Run("dial published after flush is closed without any lookup", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		late := newTestPeer("p1")
		dialStarted := make(chan struct{})
		releaseDial, doReleaseDial := newRelease()
		// released on every exit, so a failing assertion fails instead of
		// hanging fx.Finish behind the paused hook
		defer doReleaseDial()
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			close(dialStarted)
			<-releaseDial // ignores the cancel old.Close sends
			return late, nil
		}
		oldPair := p.current.Load()
		loaded := make(chan error, 1)
		go func() {
			// a background loader whose result nobody looks at
			_, err := oldPair.outgoing.Get(ctx, "p1")
			loaded <- err
		}()
		<-dialStarted
		require.NoError(t, fx.Flush(ctx))
		doReleaseDial()
		// either the dial lands in the already closed pair (the load closes
		// it and reports ErrClosed) or it is published a moment before the
		// background Close, whose pass closes it
		<-loaded
		require.Eventually(t, late.IsClosed, time.Second, 10*time.Millisecond)
		_, err := fx.Pick(ctx, "p1")
		require.Error(t, err)
		require.Eventually(t, func() bool { return oldPair.outgoing.Len() == 0 }, time.Second, 10*time.Millisecond)
		require.Equal(t, 0, p.current.Load().outgoing.Len())
		// the rejected peer still gets exactly one Closed event
		require.Eventually(t, func() bool { return len(obs.getClosed()) == 1 }, time.Second, 10*time.Millisecond)
		assert.False(t, obs.getClosed()[0].Inbound)
		require.Never(t, func() bool { return len(obs.getClosed()) > 1 }, 100*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("peer held by a declined TryClose is closed once TryClose returns", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		old := newCtlPeer("p1")
		inTryClose := make(chan struct{})
		releaseTryClose, doReleaseTryClose := newRelease()
		// released on every exit, so a failing assertion fails instead of
		// hanging fx.Finish behind the paused hook
		defer doReleaseTryClose()
		old.tryClose = func() (bool, error) {
			close(inTryClose)
			<-releaseTryClose
			return false, nil
		}
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return old, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		oldPair := p.current.Load()

		gcDone := make(chan struct{})
		go func() {
			defer close(gcDone)
			_, _ = oldPair.outgoing.TryRemove("p1")
		}()
		<-inTryClose
		require.NoError(t, fx.Flush(ctx))
		// accepted trade-off: the closer holding the entry owns the close,
		// so the peer stays open until TryClose returns; the GC then closes
		// it into the closed cache instead of restoring it
		require.Never(t, old.IsClosed, 50*time.Millisecond, 10*time.Millisecond)
		doReleaseTryClose()
		<-gcDone
		require.Eventually(t, old.IsClosed, time.Second, 10*time.Millisecond)
		require.Eventually(t, func() bool { return oldPair.outgoing.Len() == 0 }, time.Second, 10*time.Millisecond)
	})
	t.Run("concurrent waiters on a load spanning flush redial once", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()

		late := newTestPeer("p1")
		dialStarted := make(chan struct{})
		releaseDial, doReleaseDial := newRelease()
		// released on every exit, so a failing assertion fails instead of
		// hanging fx.Finish behind the paused hook
		defer doReleaseDial()
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				close(dialStarted)
				<-releaseDial
				return late, nil
			}
			// a new peer per dial, like a real dialer: a dial the replaced
			// pair cancels has its result closed, and handing that same
			// object out again would never produce an open peer
			return newTestPeer(peerId), nil
		}
		const waiters = 10
		results := make(chan peer.Peer, waiters)
		get := func() {
			pr, err := fx.Get(ctx, "p1")
			assert.NoError(t, err)
			results <- pr
		}
		go get()
		<-dialStarted
		for i := 1; i < waiters; i++ {
			go get()
		}
		require.NoError(t, fx.Flush(ctx))
		doReleaseDial()
		var fresh peer.Peer
		for i := 0; i < waiters; i++ {
			select {
			case pr := <-results:
				require.NotNil(t, pr)
				assert.NotSame(t, late, pr, "a stale peer was returned")
				if fresh == nil {
					fresh = pr
				}
				assert.Same(t, fresh, pr, "all waiters share the redial on the fresh pair")
			case <-time.After(5 * time.Second):
				t.Fatal("waiter did not return")
			}
		}
		// one redial on the fresh pair; in the window between the swap and
		// the background Close a redial can also start on the replaced pair,
		// which that Close then cancels
		assert.GreaterOrEqual(t, dials.Load(), int32(2))
		assert.LessOrEqual(t, dials.Load(), int32(3))
		require.Eventually(t, late.IsClosed, time.Second, 10*time.Millisecond)
		assert.False(t, fresh.IsClosed())
	})
	t.Run("get issued after flush does not wait on the pre-flush dial", func(t *testing.T) {
		// the gap the generation design had: a Get after Flush joined the
		// in-flight pre-flush dial and waited it out
		fx := newFixture(t)
		defer fx.Finish()

		var dials atomic.Int32
		releaseDial, doReleaseDial := newRelease()
		defer doReleaseDial()
		var cancelled atomic.Bool
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				// a dial on a dead path: returns only when cancelled
				select {
				case <-releaseDial:
				case <-ctx.Done():
					cancelled.Store(true)
					return nil, ctx.Err()
				}
			}
			return newTestPeer(peerId), nil
		}
		first := make(chan error, 1)
		go func() {
			_, err := fx.Get(ctx, "p1")
			first <- err
		}()
		require.Eventually(t, func() bool { return dials.Load() == 1 }, time.Second, time.Millisecond)
		require.NoError(t, fx.Flush(ctx))

		gctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		start := time.Now()
		pr, err := fx.Get(gctx, "p1")
		require.NoError(t, err)
		require.NotNil(t, pr)
		require.Less(t, time.Since(start), 500*time.Millisecond, "waited on the stale dial")
		// the old pair cancelled the stale dial and the first caller
		// redialed too, sharing the fresh peer
		require.NoError(t, <-first)
		require.True(t, cancelled.Load())
		require.Equal(t, int32(2), dials.Load())
	})
	t.Run("live peer returned by the replaced pair is retried on the current one", func(t *testing.T) {
		// the post-check: between the swap and old.Close the old pair still
		// hands out live pre-flush peers; a lookup that took its result from
		// there must not return it
		fx := newFixture(t)
		defer fx.Finish()

		old := newCtlPeer("p1")
		inIsClosed := make(chan struct{})
		releaseIsClosed, doReleaseIsClosed := newRelease()
		defer doReleaseIsClosed()
		old.isClosedHook = func(call int32) {
			if call == 1 {
				close(inIsClosed)
				<-releaseIsClosed
			}
		}
		// the teardown of old is held too, so it stays a live stale peer
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		old.closeHook = func(int32) { <-releaseClose }
		fresh := newTestPeer("p1")
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				return old, nil
			}
			return fresh, nil
		}
		res := make(chan peer.Peer, 1)
		go func() {
			pr, err := fx.Get(ctx, "p1")
			assert.NoError(t, err)
			res <- pr
		}()
		// the lookup has old in hand and is about to return it
		<-inIsClosed
		require.NoError(t, fx.Flush(ctx))
		doReleaseIsClosed()
		select {
		case pr := <-res:
			require.Equal(t, peer.Peer(fresh), pr)
		case <-time.After(2 * time.Second):
			t.Fatal("Get did not return")
		}
		require.Equal(t, int32(2), dials.Load())
		require.False(t, old.testPeer.IsClosed())

		// the same for Pick: nothing from the old pair
		old2 := newCtlPeer("p2")
		inIsClosed2 := make(chan struct{})
		releaseIsClosed2, doReleaseIsClosed2 := newRelease()
		defer doReleaseIsClosed2()
		old2.isClosedHook = func(call int32) {
			if call == 1 {
				close(inIsClosed2)
				<-releaseIsClosed2
			}
		}
		old2.closeHook = func(int32) { <-releaseClose }
		require.NoError(t, fx.AddPeer(ctx, old2))
		pick := make(chan error, 1)
		go func() {
			_, err := fx.Pick(ctx, "p2")
			pick <- err
		}()
		<-inIsClosed2
		require.NoError(t, fx.Flush(ctx))
		doReleaseIsClosed2()
		require.ErrorIs(t, <-pick, ocache.ErrNotExists)
	})
	t.Run("get on a closed peer whose teardown blocks returns within ctx", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		// a dead peer still in the cache (its watcher has not run yet) whose
		// Close never returns
		dead := newCtlPeer("p1")
		close(dead.closed)
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		dead.closeHook = func(int32) { <-releaseClose }
		require.NoError(t, p.current.Load().outgoing.Add("p1", dead))

		// the teardown hangs: Get must not run or wait on it past its own
		// deadline
		done := make(chan error, 1)
		go func() {
			gctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()
			_, gErr := fx.Get(gctx, "p1")
			done <- gErr
		}()
		select {
		case err := <-done:
			require.ErrorIs(t, err, context.DeadlineExceeded)
		case <-time.After(2 * time.Second):
			t.Fatal("Get waited on the blocked close")
		}
	})
	t.Run("flush racing AddPeer never serves a stale peer", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		for i := 0; i < 100; i++ {
			id := fmt.Sprintf("p%d", i)
			tp := newTestPeer(id)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				assert.NoError(t, fx.AddPeer(ctx, tp))
			}()
			go func() {
				defer wg.Done()
				_ = fx.Flush(ctx)
			}()
			wg.Wait()
			// with no lookup at all, the peer is either current or closed
			require.Eventually(t, func() bool { return tp.IsClosed() || inCurrent(p, tp) }, time.Second, time.Millisecond)
			pr, err := fx.Pick(ctx, id)
			if err == nil {
				// served only from the current pair, which the flush left alone
				require.Equal(t, peer.Peer(tp), pr)
				require.True(t, inCurrent(p, tp))
				require.False(t, tp.IsClosed())
			} else {
				// added to the replaced pair: closed by the flush
				require.Eventually(t, tp.IsClosed, time.Second, time.Millisecond)
			}
		}
	})
	t.Run("flush keeps a peer added after it", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		old := newTestPeer("old")
		require.NoError(t, fx.AddPeer(ctx, old))
		require.NoError(t, fx.Flush(ctx))
		cur := newTestPeer("cur")
		require.NoError(t, fx.AddPeer(ctx, cur))
		require.NoError(t, fx.Flush(ctx))
		require.Eventually(t, old.IsClosed, time.Second, 10*time.Millisecond)
		// added after the first flush, it is stale for the second: closed;
		// a peer added after the last flush is untouched
		require.Eventually(t, cur.IsClosed, time.Second, 10*time.Millisecond)
		last := newTestPeer("last")
		require.NoError(t, fx.AddPeer(ctx, last))
		require.Never(t, last.IsClosed, 100*time.Millisecond, 10*time.Millisecond)
		pr, err := fx.Pick(ctx, "last")
		require.NoError(t, err)
		assert.Equal(t, peer.Peer(last), pr)
	})
	t.Run("incompatible version verdict survives flush", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		eo := &errObject{id: "p1", err: handshake.ErrIncompatibleVersion, createdTime: atomic2.NewTime(time.Now())}
		require.NoError(t, p.current.Load().outgoing.Add("p1", eo))
		require.NoError(t, fx.Flush(ctx))
		require.NoError(t, fx.Flush(ctx))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			t.Error("must not redial an incompatible peer")
			return nil, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
		_, err = fx.Pick(ctx, "p1")
		require.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
		// the same verdict object, so its backoff clock is not reset
		v, err := p.current.Load().outgoing.Pick(ctx, "p1")
		require.NoError(t, err)
		require.Same(t, eo, v)
	})
	t.Run("cached dial error is dropped by flush", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		require.NoError(t, p.current.Load().outgoing.Add("p1", &errObject{id: "p1", err: assert.AnError, createdTime: atomic2.NewTime(time.Now())}))
		_, err := fx.Pick(ctx, "p1")
		require.ErrorIs(t, err, assert.AnError)

		require.NoError(t, fx.Flush(ctx))
		require.Equal(t, 0, p.current.Load().outgoing.Len())
		fresh := newTestPeer("p1")
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return fresh, nil
		}
		pr, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		assert.Equal(t, peer.Peer(fresh), pr)
	})
	t.Run("incoming replacement via AddPeer after flush", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()

		old := newTestPeer("p1")
		require.NoError(t, fx.AddPeer(ctx, old))
		require.NoError(t, fx.Flush(ctx))
		_, err := fx.Pick(ctx, "p1")
		require.Error(t, err)
		require.Eventually(t, old.IsClosed, time.Second, 10*time.Millisecond)

		// the peer reconnects: the new incoming peer lands in the fresh pair
		// at once, whatever the old pair's teardown is still doing
		repl := newTestPeer("p1")
		require.NoError(t, fx.AddPeer(ctx, repl))
		pr, err := fx.Pick(ctx, "p1")
		require.NoError(t, err)
		assert.Equal(t, peer.Peer(repl), pr)
		pr, err = fx.Get(ctx, "p1")
		require.NoError(t, err)
		assert.Equal(t, peer.Peer(repl), pr)
		require.Eventually(t, func() bool { return len(obs.getClosed()) == 1 }, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.getClosed()) > 1 }, 100*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("replacement installed before old cleanup finishes survives", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		old := newCtlPeer("p1")
		inFirstClose := make(chan struct{})
		releaseFirstClose, doReleaseFirstClose := newRelease()
		// released on every exit, so a failing assertion fails instead of
		// hanging fx.Finish behind the paused hook
		defer doReleaseFirstClose()
		old.closeHook = func(call int32) {
			// the flush's background close is the first one: hold it, so the
			// old pair's teardown overlaps the replacement
			if call == 1 {
				close(inFirstClose)
				<-releaseFirstClose
			}
		}
		require.NoError(t, fx.AddPeer(ctx, old))
		require.NoError(t, fx.Flush(ctx))
		<-inFirstClose

		repl := newTestPeer("p1")
		require.NoError(t, fx.AddPeer(ctx, repl))
		doReleaseFirstClose()

		require.Eventually(t, old.IsClosed, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool {
			pk, err := fx.Pick(ctx, "p1")
			return err != nil || pk != peer.Peer(repl)
		}, 200*time.Millisecond, 10*time.Millisecond)
		require.False(t, repl.IsClosed())
		require.Equal(t, 1, p.current.Load().incoming.Len())
	})
	t.Run("repeated flush closes every replaced pair once", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		tr := trackPairs(p)

		var peers []*testPeer
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			tp := newTestPeer(peerId)
			peers = append(peers, tp)
			return tp, nil
		}
		for i := 0; i < 3; i++ {
			pr, err := fx.Get(ctx, "p1")
			require.NoError(t, err)
			require.Equal(t, peer.Peer(peers[len(peers)-1]), pr)
			require.NoError(t, fx.Flush(ctx))
			require.NoError(t, fx.Flush(ctx))
		}
		require.Len(t, peers, 3)
		for _, tp := range peers {
			require.Eventually(t, tp.IsClosed, time.Second, 10*time.Millisecond)
		}
		pairs := tr.all()
		require.Len(t, pairs, 7)
		cur := p.current.Load()
		require.Same(t, pairs[6], cur)
		for _, c := range pairs[:6] {
			require.Eventually(t, c.isClosed, time.Second, 10*time.Millisecond)
		}
		// the current pair is open and empty
		require.Equal(t, 0, cur.outgoing.Len())
		pr, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		require.Equal(t, peer.Peer(peers[3]), pr)
	})
	t.Run("concurrent flushes never leak a pair", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		tr := trackPairs(p)

		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.NoError(t, fx.Flush(ctx))
			}()
		}
		wg.Wait()
		pairs := tr.all()
		require.Len(t, pairs, 51)
		cur := p.current.Load()
		require.Same(t, pairs[50], cur)
		for _, c := range pairs[:50] {
			require.Eventually(t, c.isClosed, time.Second, 10*time.Millisecond)
		}
		require.False(t, cur.isClosed())
	})
	t.Run("flush then shutdown", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		p := fx.Service.(*poolService).pool
		tr := trackPairs(p)
		in := newTestPeer("in")
		out := newCtlPeer("out")
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return out, nil
		}
		require.NoError(t, fx.AddPeer(ctx, in))
		_, err := fx.Get(ctx, "out")
		require.NoError(t, err)
		require.NoError(t, fx.Flush(ctx))
		fx.Finish()
		require.Eventually(t, in.IsClosed, time.Second, 10*time.Millisecond)
		require.Eventually(t, out.IsClosed, time.Second, 10*time.Millisecond)
		// a flush on a closed pool is a no-op: no pair is built, the closed
		// pair stays current and lookups fail with ErrClosed
		cur := p.current.Load()
		require.NoError(t, fx.Flush(ctx))
		require.Equal(t, 2, tr.created())
		require.Same(t, cur, p.current.Load())
		for _, c := range tr.all() {
			require.True(t, c.isClosed())
		}
		_, err = fx.Pick(ctx, "in")
		require.ErrorIs(t, err, ocache.ErrClosed)
		_, err = fx.Get(ctx, "out")
		require.ErrorIs(t, err, ocache.ErrClosed)
		require.ErrorIs(t, fx.AddPeer(ctx, newTestPeer("late")), ocache.ErrClosed)
		// idempotent
		require.NoError(t, fx.Service.Close(ctx))
	})
	t.Run("close waits for the pairs a flush is still closing", func(t *testing.T) {
		fx := newFixture(t)
		slow := newCtlPeer("p1")
		slow.closeHook = func(int32) { time.Sleep(100 * time.Millisecond) }
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return slow, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		require.NoError(t, fx.Flush(ctx))
		fx.Finish()
		// no teardown outlives the pool
		require.True(t, slow.IsClosed())
	})
	t.Run("close is bounded when a flushed pair never finishes closing", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 100 * time.Millisecond
		})
		hung := newCtlPeer("p1")
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		hung.closeHook = func(int32) { <-releaseClose }
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return hung, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		require.NoError(t, fx.Flush(ctx))
		start := time.Now()
		fx.Finish()
		require.Less(t, time.Since(start), 2*time.Second)
		require.False(t, hung.IsClosed())
	})
	t.Run("hung peer close does not hold back the others", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 300 * time.Millisecond
		})
		defer fx.Finish()

		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		peers := map[string]*ctlPeer{}
		for _, id := range []string{"p1", "p2", "p3", "p4"} {
			peers[id] = newCtlPeer(id)
		}
		peers["p1"].closeHook = func(int32) { <-releaseClose }
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return peers[peerId], nil
		}
		for id := range peers {
			_, err := fx.Get(ctx, id)
			require.NoError(t, err)
		}
		require.NoError(t, fx.Flush(ctx))
		// well within closeTimeout, so not thanks to the pass giving up on p1
		require.Eventually(t, func() bool {
			return peers["p2"].IsClosed() && peers["p3"].IsClosed() && peers["p4"].IsClosed()
		}, 100*time.Millisecond, time.Millisecond)
		require.False(t, peers["p1"].IsClosed())
		doReleaseClose()
		require.Eventually(t, peers["p1"].IsClosed, time.Second, 10*time.Millisecond)
		// the parallel close and the cache's own pass both reach each peer:
		// the second call is the idempotent no-op the pool relies on
		for _, pr := range peers {
			require.LessOrEqual(t, pr.closeCalls.Load(), int32(2), pr.Id())
		}
	})
	t.Run("hung incoming close does not delay cancelling a pre-flush dial", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 300 * time.Millisecond
		})
		defer fx.Finish()
		in := newCtlPeer("in")
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		in.closeHook = func(int32) { <-releaseClose }
		require.NoError(t, fx.AddPeer(ctx, in))

		var dials atomic.Int32
		var cancelled atomic.Bool
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				// a dead path: returns only when cancelled
				<-ctx.Done()
				cancelled.Store(true)
				return nil, ctx.Err()
			}
			return newTestPeer(peerId), nil
		}
		type result struct {
			pr  peer.Peer
			err error
		}
		first := make(chan result, 1)
		go func() {
			gctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			pr, err := fx.Get(gctx, "dead")
			first <- result{pr, err}
		}()
		require.Eventually(t, func() bool { return dials.Load() == 1 }, time.Second, time.Millisecond)
		start := time.Now()
		require.NoError(t, fx.Flush(ctx))
		res := <-first
		require.NoError(t, res.err)
		require.False(t, res.pr.IsClosed())
		require.Less(t, time.Since(start), 500*time.Millisecond, "pre-flush Get waited out the dead dial")
		require.True(t, cancelled.Load())
		require.Equal(t, int32(2), dials.Load())
	})
	t.Run("hung incoming close does not keep a late dial alive", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 300 * time.Millisecond
		})
		defer fx.Finish()
		in := newCtlPeer("in")
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		in.closeHook = func(int32) { <-releaseClose }
		require.NoError(t, fx.AddPeer(ctx, in))

		late := newTestPeer("out")
		var dials atomic.Int32
		var cancelled atomic.Bool
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				// a slow handshake that completes despite the cancellation
				select {
				case <-ctx.Done():
					cancelled.Store(true)
				case <-time.After(400 * time.Millisecond):
				}
				return late, nil
			}
			return newTestPeer(peerId), nil
		}
		first := make(chan peer.Peer, 1)
		go func() {
			gctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			pr, err := fx.Get(gctx, "out")
			assert.NoError(t, err)
			first <- pr
		}()
		require.Eventually(t, func() bool { return dials.Load() == 1 }, time.Second, time.Millisecond)
		start := time.Now()
		require.NoError(t, fx.Flush(ctx))
		pr := <-first
		require.NotSame(t, late, pr)
		require.Less(t, time.Since(start), 300*time.Millisecond, "pre-flush Get waited out the stale dial")
		require.True(t, cancelled.Load())
		// published into the old outgoing cache, which closed at once
		require.Eventually(t, late.IsClosed, time.Second, 10*time.Millisecond)
		require.Equal(t, int32(2), dials.Load())
	})
	t.Run("lookup blocked behind a GC TryClose on the replaced pair retries at once", func(t *testing.T) {
		// nothing in the old pair's Close can cut this wait short (the GC
		// owns the entry): only the pair ctx that the swap cancels does
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 300 * time.Millisecond
		})
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		old := newCtlPeer("p1")
		inTryClose := make(chan struct{})
		releaseTryClose, doReleaseTryClose := newRelease()
		defer doReleaseTryClose()
		old.tryClose = func() (bool, error) {
			close(inTryClose)
			<-releaseTryClose
			return false, nil
		}
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				return old, nil
			}
			return newTestPeer(peerId), nil
		}
		_, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		oldPair := p.current.Load()
		go func() { _, _ = oldPair.outgoing.TryRemove("p1") }()
		<-inTryClose

		got := make(chan peer.Peer, 1)
		go func() {
			gctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			pr, err := fx.Get(gctx, "p1")
			assert.NoError(t, err)
			got <- pr
		}()
		// the Get is parked on the closing entry
		require.Never(t, func() bool { return len(got) > 0 }, 50*time.Millisecond, 5*time.Millisecond)
		start := time.Now()
		require.NoError(t, fx.Flush(ctx))
		select {
		case pr := <-got:
			require.NotSame(t, old, pr)
			require.Less(t, time.Since(start), 500*time.Millisecond)
		case <-time.After(2 * time.Second):
			t.Fatal("Get stayed parked on the replaced pair")
		}
	})
	t.Run("GetOneOf never returns a peer from the replaced pair", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		old := newCtlPeer("g")
		fresh := newTestPeer("g")
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if dials.Add(1) == 1 {
				return old, nil
			}
			return fresh, nil
		}
		_, err := fx.Get(ctx, "g")
		require.NoError(t, err)
		// keep old live so the retry, not its close, decides the result
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		old.closeHook = func(int32) { <-releaseClose }
		paused := make(chan struct{})
		releaseIsClosed, doReleaseIsClosed := newRelease()
		defer doReleaseIsClosed()
		var armed atomic.Bool
		armed.Store(true)
		old.isClosedHook = func(int32) {
			if armed.CompareAndSwap(true, false) {
				close(paused)
				<-releaseIsClosed
			}
		}
		got := make(chan peer.Peer, 1)
		go func() {
			pr, err := fx.GetOneOf(ctx, []string{"g"})
			assert.NoError(t, err)
			got <- pr
		}()
		// getIfActive has old in hand
		<-paused
		require.NoError(t, fx.Flush(ctx))
		doReleaseIsClosed()
		require.Same(t, fresh, <-got)
	})
	t.Run("add in flight across flush is never rejected", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		tp := newCtlPeer("p1")
		inAdd := make(chan struct{})
		releaseAdd, doReleaseAdd := newRelease()
		defer doReleaseAdd()
		tp.idHook = func(call int32) {
			if call == 1 {
				close(inAdd)
				<-releaseAdd
			}
		}
		added := make(chan error, 1)
		go func() { added <- fx.AddPeer(ctx, tp) }()
		<-inAdd
		flushed := make(chan struct{})
		go func() {
			_ = fx.Flush(ctx)
			close(flushed)
		}()
		// the swap waits for the add that already picked its pair, so the
		// peer can neither land in a closed cache nor be dropped
		select {
		case <-flushed:
			t.Fatal("Flush did not wait for the add in flight")
		case <-time.After(50 * time.Millisecond):
		}
		doReleaseAdd()
		require.NoError(t, <-added)
		<-flushed
		// accepted before the flush completed: closed by it
		require.Eventually(t, tp.IsClosed, time.Second, 10*time.Millisecond)
	})
	t.Run("get parked in a dial gets ErrClosed when the pool closes", func(t *testing.T) {
		fx := newFixture(t)
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		got := make(chan error, 1)
		go func() {
			_, err := fx.Get(ctx, "x")
			got <- err
		}()
		require.Eventually(t, func() bool { return dials.Load() == 1 }, time.Second, time.Millisecond)
		fx.Finish()
		// the pair was cancelled but not replaced: the pool is closing, there
		// is nothing to retry on
		require.ErrorIs(t, <-got, ocache.ErrClosed)
	})
	t.Run("a flush landing between the lookup and its post-check is a retry, not a closed pool", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// pairs built from here on carry the seam
		var armed atomic.Bool
		inErr := make(chan struct{})
		releaseErr, doReleaseErr := newRelease()
		defer doReleaseErr()
		orig := p.newCaches
		p.newCaches = func() *caches {
			c := orig()
			c.ctx = &hookCtx{Context: c.ctx, onErr: func() {
				if armed.CompareAndSwap(true, false) {
					close(inErr)
					<-releaseErr
				}
			}}
			return c
		}
		require.NoError(t, fx.Flush(ctx))
		// a cached verdict takes the slow path (the fast path only serves peers)
		eo := &errObject{id: "inc", err: handshake.ErrIncompatibleVersion, createdTime: atomic2.NewTime(time.Now())}
		require.NoError(t, p.current.Load().outgoing.Add("inc", eo))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			t.Error("must not redial an incompatible peer")
			return nil, nil
		}
		armed.Store(true)
		got := make(chan error, 1)
		go func() {
			_, err := fx.Get(ctx, "inc")
			got <- err
		}()
		// f has returned; lookup is reading the pair state
		<-inErr
		require.NoError(t, fx.Flush(ctx))
		doReleaseErr()
		// the pair is cancelled and replaced: a retry on the fresh pair, which
		// carries the verdict, never ErrClosed
		require.ErrorIs(t, <-got, handshake.ErrIncompatibleVersion)
	})
	t.Run("add whose duplicate is flushed between Add and Pick retries on the current pair", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		require.NoError(t, fx.AddPeer(ctx, newTestPeer("d")))
		repl := newCtlPeer("d")
		old := p.current.Load()
		repl.idHook = func(call int32) {
			if call == 2 {
				// the Pick after ErrExists: the pair holding the duplicate
				// is replaced and closed under AddPeer's feet
				assert.NoError(t, fx.Flush(ctx))
				assert.Eventually(t, old.isClosed, time.Second, time.Millisecond)
			}
		}
		require.NoError(t, fx.AddPeer(ctx, repl))
		require.True(t, inCurrent(p, repl))
		require.False(t, repl.IsClosed())
	})
	t.Run("add racing a Get on the same id succeeds", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// a pair whose incoming loader can be held open, standing in for the
		// instant ErrNotExists one: the Get leaves a loading entry that Add
		// trips over and Pick waits out
		gate := make(chan struct{})
		entered := make(chan struct{})
		var once sync.Once
		orig := p.newCaches
		p.newCaches = func() *caches {
			c := orig()
			_ = c.incoming.Close()
			c.incoming = ocache.New(func(ctx context.Context, id string) (ocache.Object, error) {
				once.Do(func() { close(entered) })
				<-gate
				return nil, ocache.ErrNotExists
			}, ocache.WithGCPeriod(0))
			c.peekIncoming = mustPeeker(c.incoming)
			return c
		}
		require.NoError(t, fx.Flush(ctx))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return newTestPeer(peerId), nil
		}
		go func() { _, _ = fx.Get(ctx, "d") }()
		<-entered
		tp := newTestPeer("d")
		added := make(chan error, 1)
		go func() { added <- fx.AddPeer(ctx, tp) }()
		require.Never(t, func() bool { return len(added) > 0 }, 50*time.Millisecond, 5*time.Millisecond)
		close(gate)
		require.NoError(t, <-added)
		require.True(t, inCurrent(p, tp))
	})
	t.Run("close waits for the parallel peer closes of every pair", func(t *testing.T) {
		for _, flushed := range []bool{true, false} {
			fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
				ps.closeTimeout = 5 * time.Second
			})
			pr := newCtlPeer("p1")
			var finished atomic.Bool
			pr.closeHook = func(int32) {
				// only the parallel close is slow; the cache's own pass
				// closes the peer a second time and returns at once
				if fromClosePrepass() {
					time.Sleep(200 * time.Millisecond)
					finished.Store(true)
				}
			}
			fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
				return pr, nil
			}
			_, err := fx.Get(ctx, "p1")
			require.NoError(t, err)
			if flushed {
				require.NoError(t, fx.Flush(ctx))
			}
			fx.Finish()
			require.True(t, finished.Load(), "flushed=%v: Close returned before the parallel close finished", flushed)
		}
	})
	t.Run("close is bounded by its ctx with a hung peer", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 5 * time.Second
		})
		hung := newCtlPeer("p1")
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		hung.closeHook = func(int32) { <-releaseClose }
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return hung, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		cctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		require.NoError(t, fx.Service.Close(cctx))
		require.Less(t, time.Since(start), time.Second)
		require.False(t, hung.IsClosed())
	})
	t.Run("hung incoming close does not hold back the other incoming peers", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 300 * time.Millisecond
		})
		defer fx.Finish()
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		hung := newCtlPeer("hung")
		hung.closeHook = func(int32) { <-releaseClose }
		require.NoError(t, fx.AddPeer(ctx, hung))
		var others []*testPeer
		for i := 0; i < 10; i++ {
			tp := newTestPeer(fmt.Sprintf("in%d", i))
			others = append(others, tp)
			require.NoError(t, fx.AddPeer(ctx, tp))
		}
		require.NoError(t, fx.Flush(ctx))
		// well within closeTimeout, so not thanks to the pass giving up
		require.Eventually(t, func() bool {
			for _, tp := range others {
				if !tp.IsClosed() {
					return false
				}
			}
			return true
		}, 100*time.Millisecond, time.Millisecond)
		require.False(t, hung.IsClosed())
	})
	t.Run("connected and closed pairing for flushed peers", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		in := newTestPeer("in")
		out := newTestPeer("out")
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return out, nil
		}
		require.NoError(t, fx.AddPeer(ctx, in))
		_, err := fx.Get(ctx, "out")
		require.NoError(t, err)
		// both a flush and a later lookup try to discard them: each peer
		// still reports a single Closed event
		require.NoError(t, fx.Flush(ctx))
		_, _ = fx.Pick(ctx, "in")
		_, _ = fx.Pick(ctx, "out")
		require.NoError(t, fx.Flush(ctx))

		require.Eventually(t, func() bool { return len(obs.getClosed()) == 2 }, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.getClosed()) > 2 }, 100*time.Millisecond, 10*time.Millisecond)
		byInbound := map[bool]peerobserver.Event{}
		for _, ev := range obs.getClosed() {
			byInbound[ev.Inbound] = ev
		}
		assert.Equal(t, "in", byInbound[true].PeerId)
		assert.Equal(t, "out", byInbound[false].PeerId)
	})
}

// taggedPeer carries the number of the pair that was current when it was dialed
type taggedPeer struct {
	*testPeer
	tag int64
}

func TestPool_FlushStorm(t *testing.T) {
	t.Run("back-to-back flushes never fail a lookup or serve a stale peer", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = time.Second
		})
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		tr := trackPairs(p)

		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			// shorter than the flush period, or no dial would ever complete
			// before the next swap (a livelock both designs share)
			time.Sleep(200 * time.Microsecond)
			return &taggedPeer{testPeer: newTestPeer(peerId), tag: tr.seqOf(p.current.Load())}, nil
		}
		// the workers run until the flusher has done wantFlushes (about
		// 300ms on an idle machine; with GOMAXPROCS=1 the busy workers starve
		// it, so a fixed duration would not do)
		const wantFlushes = 60
		flushes, stopFlusher := startFlusher(t, fx, time.Millisecond)
		defer stopFlusher()
		cap := time.Now().Add(20 * time.Second)
		var stale, failed, ok atomic.Int32
		var wg sync.WaitGroup
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; flushes.Load() < wantFlushes && time.Now().Before(cap); i++ {
					before := tr.seqOf(p.current.Load())
					gctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					pr, err := fx.Get(gctx, "a")
					switch {
					case err != nil:
						failed.Add(1)
						t.Errorf("Get failed while the pool is open: %v", err)
					case pr.(*taggedPeer).tag < before:
						// a peer from a pair replaced before the Get started
						stale.Add(1)
					default:
						ok.Add(1)
					}
					if i%50 == 0 {
						// Pick and GetOneOf take the same path
						if _, err = fx.Pick(gctx, "a"); err != nil {
							assert.NotErrorIs(t, err, ocache.ErrClosed)
						}
						if _, err = fx.GetOneOf(gctx, []string{"a"}); err != nil {
							t.Errorf("GetOneOf failed while the pool is open: %v", err)
						}
					}
					cancel()
				}
			}()
		}
		wg.Wait()
		stopFlusher()
		t.Logf("flushes=%d ok=%d failed=%d stale=%d", flushes.Load(), ok.Load(), failed.Load(), stale.Load())
		require.GreaterOrEqual(t, flushes.Load(), int64(wantFlushes))
		require.Greater(t, ok.Load(), int32(100))
		require.Zero(t, failed.Load())
		require.Zero(t, stale.Load())
	})
	t.Run("peers added during swaps land in the current pair and stay open", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = time.Second
		})
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		_, stopFlusher := startFlusher(t, fx, time.Millisecond)
		defer stopFlusher()
		var peers []*testPeer
		for i := 0; i < 500; i++ {
			tp := newTestPeer(fmt.Sprintf("p%d", i))
			peers = append(peers, tp)
			// never rejected with ErrClosed while the pool is open
			require.NoError(t, fx.AddPeer(ctx, tp))
			if i%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		stopFlusher()
		// every peer is now either in the current pair, open, or was
		// flushed and closed; none is both current and killed
		for _, tp := range peers {
			require.Eventually(t, func() bool { return tp.IsClosed() || inCurrent(p, tp) }, time.Second, time.Millisecond, tp.Id())
			if inCurrent(p, tp) {
				require.False(t, tp.IsClosed(), tp.Id())
			}
		}
		// with the swaps over, a new peer is current and stays open
		last := newTestPeer("last")
		require.NoError(t, fx.AddPeer(ctx, last))
		require.True(t, inCurrent(p, last))
		require.Never(t, last.IsClosed, 50*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("a dial that never completes under a flush storm ends with the ctx error", func(t *testing.T) {
		// every swap cancels the lookup's dial and the retry starts another:
		// the livelock is bounded by the caller's ctx and is not reported
		// as a closed pool
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = time.Second
		})
		defer fx.Finish()
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		_, stopFlusher := startFlusher(t, fx, 2*time.Millisecond)
		defer stopFlusher()
		got := make(chan error, 1)
		go func() {
			gctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel()
			_, err := fx.Get(gctx, "s")
			got <- err
		}()
		select {
		case err := <-got:
			require.ErrorIs(t, err, context.DeadlineExceeded)
		case <-time.After(5 * time.Second):
			t.Fatal("Get did not return with its ctx")
		}
		stopFlusher()
		require.Greater(t, dials.Load(), int32(1), "the dial was never retried")
	})
	t.Run("a kept verdict is never redialed while flushes run", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		eo := &errObject{id: "inc", err: handshake.ErrIncompatibleVersion, createdTime: atomic2.NewTime(time.Now())}
		require.NoError(t, p.current.Load().outgoing.Add("inc", eo))
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			return newTestPeer(peerId), nil
		}
		// the verdict must be in the fresh pair before that pair is published
		stop := make(chan struct{})
		var wg sync.WaitGroup
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					_, err := fx.Get(ctx, "inc")
					assert.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
				}
			}()
		}
		for i := 0; i < 300; i++ {
			require.NoError(t, fx.Flush(ctx))
		}
		close(stop)
		wg.Wait()
		require.Zero(t, dials.Load())
	})
}

// testMetric exposes a registry the way the metric component does; the other
// methods are never called by the pool
type testMetric struct {
	metric.Metric
	reg *prometheus.Registry
}

func (m *testMetric) Init(a *app.App) error           { return nil }
func (m *testMetric) Name() string                    { return metric.CName }
func (m *testMetric) Run(ctx context.Context) error   { return nil }
func (m *testMetric) Close(ctx context.Context) error { return nil }
func (m *testMetric) Registry() *prometheus.Registry  { return m.reg }

func TestPool_FlushMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
		a.Register(&testMetric{reg: reg})
	})
	defer fx.Finish()

	gather := func() map[string]float64 {
		families, err := reg.Gather()
		require.NoError(t, err)
		out := map[string]float64{}
		for _, mf := range families {
			require.Len(t, mf.GetMetric(), 1)
			m := mf.GetMetric()[0]
			if m.GetGauge() != nil {
				out[mf.GetName()] = m.GetGauge().GetValue()
			} else {
				out[mf.GetName()] = m.GetCounter().GetValue()
			}
		}
		return out
	}
	expectedNames := []string{
		"netpool_outgoing_hit", "netpool_outgoing_miss", "netpool_outgoing_gc", "netpool_outgoing_size",
		"netpool_incoming_hit", "netpool_incoming_miss", "netpool_incoming_gc", "netpool_incoming_size",
	}
	names := func(values map[string]float64) (out []string) {
		for name := range values {
			out = append(out, name)
		}
		return
	}
	require.ElementsMatch(t, expectedNames, names(gather()))

	// recreating the caches re-registers nothing
	for i := 0; i < 100; i++ {
		require.NoError(t, fx.Flush(ctx))
	}
	fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
		return newTestPeer(peerId), nil
	}
	_, err := fx.Get(ctx, "out")
	require.NoError(t, err)
	_, err = fx.Get(ctx, "out")
	require.NoError(t, err)
	require.NoError(t, fx.AddPeer(ctx, newTestPeer("in1")))
	require.NoError(t, fx.AddPeer(ctx, newTestPeer("in2")))
	values := gather()
	require.ElementsMatch(t, expectedNames, names(values))
	// size reads the current pair; the counters are continuous across pairs
	assert.Equal(t, float64(1), values["netpool_outgoing_size"])
	assert.Equal(t, float64(2), values["netpool_incoming_size"])
	assert.Equal(t, float64(1), values["netpool_outgoing_miss"])
	// Get tries incoming first (a miss), then hits outgoing on the second call
	assert.Equal(t, float64(1), values["netpool_outgoing_hit"])
	assert.Equal(t, float64(2), values["netpool_incoming_miss"])

	require.NoError(t, fx.Flush(ctx))
	values = gather()
	assert.Equal(t, float64(0), values["netpool_outgoing_size"])
	assert.Equal(t, float64(0), values["netpool_incoming_size"])
	assert.Equal(t, float64(1), values["netpool_outgoing_miss"])
}

func TestPool_FlushGoroutines(t *testing.T) {
	// every pair owns two GC tickers and every flush a closer goroutine: all
	// of them must be gone once the pool is closed
	runtime.GC()
	before := runtime.NumGoroutine()
	fx := newFixture(t)
	fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
		return newTestPeer(peerId), nil
	}
	for i := 0; i < 30; i++ {
		_, err := fx.Get(ctx, "out")
		require.NoError(t, err)
		require.NoError(t, fx.AddPeer(ctx, newTestPeer("in")))
		require.NoError(t, fx.Flush(ctx))
	}
	fx.Finish()
	// polled from this goroutine so the baseline is comparable (Eventually
	// would add its own)
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines before=%d after=%d", before, runtime.NumGoroutine())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
