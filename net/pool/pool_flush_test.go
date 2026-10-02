package pool

import (
	"context"
	"fmt"
	net2 "net"
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
	"storj.io/drpc"

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
	if err != nil {
		return false
	}
	got, err := getPeer(v)
	return err == nil && got == pr
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
	// closePeer exists for this: it is the one frame the pre-close has and
	// the cache's own pass has not
	return strings.Contains(string(buf[:n]), "pool.closePeer")
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
		require.Less(t, time.Since(start), 2*time.Second, "waited on the stale dial")
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
	t.Run("every cached verdict is carried over by flush", func(t *testing.T) {
		// the loader caches incompatible-version verdicts only, so Flush
		// carries whatever errObject it finds, without inspecting it
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		eo := &errObject{id: "p1", err: assert.AnError, createdTime: atomic2.NewTime(time.Now())}
		require.NoError(t, p.current.Load().outgoing.Add("p1", eo))
		require.NoError(t, fx.Flush(ctx))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			t.Error("must not redial a peer with a cached verdict")
			return nil, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.ErrorIs(t, err, assert.AnError)
		v, err := p.current.Load().outgoing.Pick(ctx, "p1")
		require.NoError(t, err)
		require.Same(t, eo, v)
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
		require.Less(t, time.Since(start), 2*time.Second, "pre-flush Get waited out the dead dial")
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
				case <-time.After(10 * time.Second):
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
		require.Less(t, time.Since(start), 2*time.Second, "pre-flush Get waited out the stale dial")
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
			require.Less(t, time.Since(start), 2*time.Second)
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
	t.Run("get never loads into the incoming cache", func(t *testing.T) {
		// incoming entries come from AddPeer alone: a Get that finds none
		// must not leave a loading entry behind for an AddPeer to trip over
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		var loads atomic.Int32
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onGet: func() { loads.Add(1) }}
		})
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return newTestPeer(peerId), nil
		}
		for i := 0; i < 50; i++ {
			id := fmt.Sprintf("p%d", i)
			tp := newTestPeer(id)
			done := make(chan struct{})
			go func() {
				_, _ = fx.Get(ctx, id)
				close(done)
			}()
			require.NoError(t, fx.AddPeer(ctx, tp))
			<-done
			require.True(t, inCurrent(p, tp))
		}
		require.Zero(t, loads.Load(), "Get loaded into the incoming cache")
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
		// the teardown finishes once the peer lets it; a second Close is a no-op
		doReleaseClose()
		fx.Finish()
		require.Eventually(t, hung.IsClosed, time.Second, 10*time.Millisecond)
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
	t.Run("add during shutdown returns ErrClosed without spinning", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		require.NoError(t, fx.AddPeer(ctx, newTestPeer("d")))
		// shutdown has begun: nothing is evicted any more, so the duplicate
		// can never be added; it must not be retried either
		p.closingCancel()
		dup := newCtlPeer("d")
		got := make(chan error, 1)
		go func() { got <- fx.AddPeer(context.Background(), dup) }()
		select {
		case err := <-got:
			require.ErrorIs(t, err, ocache.ErrClosed)
		case <-time.After(2 * time.Second):
			t.Fatal("AddPeer did not return")
		}
		require.LessOrEqual(t, dup.idCall.Load(), int32(2), "AddPeer kept retrying")
	})
	t.Run("add never waits on a flushed pair's teardown", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = time.Second
		})
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		old := newCtlPeer("d")
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		old.closeHook = func(int32) { <-releaseClose }
		require.NoError(t, fx.AddPeer(ctx, old))
		repl := newCtlPeer("d")
		repl.idHook = func(call int32) {
			if call == 2 {
				// between ErrExists and the Pick: the pair holding old is
				// replaced; old's hung teardown is now the flush's problem
				assert.NoError(t, fx.Flush(ctx))
			}
		}
		got := make(chan error, 1)
		go func() { got <- fx.AddPeer(context.Background(), repl) }()
		select {
		case err := <-got:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("AddPeer waited on the old pair's teardown")
		}
		require.True(t, inCurrent(p, repl))
		require.False(t, repl.IsClosed())
	})
	t.Run("a live incoming arriving while a dead outgoing is evicted is used instead of a dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		dead := newCtlPeer("p1")
		close(dead.closed)
		inClose := make(chan struct{})
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		dead.closeHook = func(call int32) {
			if call == 1 {
				close(inClose)
			}
			<-releaseClose
		}
		require.NoError(t, p.current.Load().outgoing.Add("p1", dead))
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			return newTestPeer(peerId), nil
		}
		got := make(chan peer.Peer, 1)
		go func() {
			gctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			pr, err := fx.Get(gctx, "p1")
			assert.NoError(t, err)
			got <- pr
		}()
		// the Get is evicting dead; the peer connects to us meanwhile
		<-inClose
		live := newTestPeer("p1")
		require.NoError(t, fx.AddPeer(ctx, live))
		doReleaseClose()
		require.Same(t, live, <-got)
		require.Zero(t, dials.Load(), "dialed although an incoming connection was available")
	})
	t.Run("flush reports Closed before it returns, once per instance, before the redial", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// Connected comes from peerservice, before the peer reaches the pool
		connected := func(id string, inbound bool) {
			p.observer.Notify(peerobserver.Event{Kind: peerobserver.KindConnected, PeerId: id, Inbound: inbound})
		}
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			connected(peerId, false)
			return newTestPeer(peerId), nil
		}
		for round := 0; round < 3; round++ {
			_, err := fx.Get(ctx, "x")
			require.NoError(t, err)
			connected("y", true)
			require.NoError(t, fx.AddPeer(ctx, newTestPeer("y")))
			require.NoError(t, fx.Flush(ctx))
			// delivered synchronously, in both directions
			require.Len(t, obs.getClosed(), 2*(round+1))
		}
		// the watchers of the flushed peers must not report them again
		require.Never(t, func() bool { return len(obs.getClosed()) > 6 }, 200*time.Millisecond, 10*time.Millisecond)
		for _, id := range []string{"x", "y"} {
			kinds := obs.kindsFor(id)
			require.Len(t, kinds, 6, id)
			for i, k := range kinds {
				want := peerobserver.KindConnected
				if i%2 == 1 {
					want = peerobserver.KindClosed
				}
				require.Equal(t, want, k, "%s: event %d", id, i)
			}
		}
	})
	t.Run("hung outgoing close does not delay closing the incoming cache", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 2 * time.Second
		})
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		hung := newCtlPeer("out")
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		hung.closeHook = func(int32) { <-releaseClose }
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return hung, nil
		}
		_, err := fx.Get(ctx, "out")
		require.NoError(t, err)
		in := newTestPeer("in")
		require.NoError(t, fx.AddPeer(ctx, in))
		oldPair := p.current.Load()
		require.NoError(t, fx.Flush(ctx))
		// well within closeTimeout: the incoming cache does not queue behind
		// the outgoing pass that is stuck on hung
		require.Eventually(t, func() bool {
			_, err := oldPair.incoming.Pick(ctx, "in")
			return err == ocache.ErrClosed && in.IsClosed()
		}, 500*time.Millisecond, time.Millisecond)
		require.False(t, hung.IsClosed())
	})
	t.Run("add gives up on an entry that cannot be evicted", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// an incoming cache that never removes anything stands in for an
		// entry some other closer keeps hold of
		orig := p.newCaches
		p.newCaches = func() *caches {
			c := orig()
			c.incoming = &stickyCache{OCache: c.incoming, peek: c.peekIncoming}
			c.peekIncoming = mustPeeker(c.incoming)
			return c
		}
		require.NoError(t, fx.Flush(ctx))
		require.NoError(t, fx.AddPeer(ctx, newTestPeer("d")))
		dup := newCtlPeer("d")
		got := make(chan error, 1)
		go func() { got <- fx.AddPeer(context.Background(), dup) }()
		select {
		case err := <-got:
			require.ErrorIs(t, err, ocache.ErrExists)
		case <-time.After(2 * time.Second):
			t.Fatal("AddPeer kept retrying an entry it cannot evict")
		}
		// one pass plus the three retries, two Id calls each (Add, Pick)
		require.Equal(t, int32(8), dup.idCall.Load())
	})
	t.Run("add waits for the old connection's close before giving up", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		old := newCtlPeer("d")
		inClose := make(chan struct{})
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		old.closeHook = func(call int32) {
			if call == 1 {
				close(inClose)
			}
			<-releaseClose
		}
		require.NoError(t, fx.AddPeer(ctx, old))
		// the connection dies: its watcher evicts it, and the close hangs
		close(old.closed)
		<-inClose
		// the remote reconnects while the old entry is still mid-close
		repl := newTestPeer("d")
		got := make(chan error, 1)
		go func() {
			actx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			got <- fx.AddPeer(actx, repl)
		}()
		require.Never(t, func() bool { return len(got) > 0 }, 100*time.Millisecond, 10*time.Millisecond)
		doReleaseClose()
		require.NoError(t, <-got)
		require.True(t, inCurrent(p, repl))
		require.False(t, repl.IsClosed())
	})
	t.Run("add parked on an old connection's close moves on when the pair is flushed", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = time.Second
		})
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		old := newCtlPeer("d")
		inClose := make(chan struct{})
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		old.closeHook = func(call int32) {
			if call == 1 {
				close(inClose)
			}
			<-releaseClose
		}
		require.NoError(t, fx.AddPeer(ctx, old))
		repl := newTestPeer("d")
		got := make(chan error, 1)
		// Accept passes a Background ctx: only the pair can end the wait
		go func() { got <- fx.AddPeer(context.Background(), repl) }()
		<-inClose
		require.NoError(t, fx.Flush(ctx))
		select {
		case err := <-got:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("AddPeer stayed parked on the flushed pair's teardown")
		}
		require.True(t, inCurrent(p, repl))
	})
	t.Run("a watcher evicting a peer while flush snapshots it reports it once", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		// the watcher's RemoveSame starts a Flush and lets it snapshot the
		// still-present peer, then removes; Flush may only mark after that
		snapshotDone := make(chan struct{})
		proceed := make(chan struct{})
		var flushed sync.WaitGroup
		var once sync.Once
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			hc := &hookedCache{OCache: inner, peek: peek}
			hc.onForEach = func() {
				once.Do(func() {
					close(snapshotDone)
					<-proceed
				})
			}
			hc.onRemoveSame = func() {
				flushed.Add(1)
				go func() {
					defer flushed.Done()
					assert.NoError(t, fx.Flush(ctx))
				}()
				<-snapshotDone
			}
			return hc
		})
		tp := newTestPeer("x")
		require.NoError(t, fx.AddPeer(ctx, tp))
		require.NoError(t, tp.Close())
		<-snapshotDone
		// the watcher's removal runs now, with Flush holding the snapshot
		time.Sleep(20 * time.Millisecond)
		close(proceed)
		flushed.Wait()
		require.Eventually(t, func() bool { return len(obs.getClosed()) == 1 }, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.getClosed()) > 1 }, 200*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("flush racing close reports nothing after shutdown began", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureCfg(t, obs, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 2 * time.Second
		})
		p := fx.Service.(*poolService).pool
		inSnapshot := make(chan struct{})
		proceed := make(chan struct{})
		var once sync.Once
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onForEach: func() {
				once.Do(func() {
					close(inSnapshot)
					<-proceed
				})
			}}
		})
		require.NoError(t, fx.AddPeer(ctx, newTestPeer("x")))
		flushed := make(chan struct{})
		go func() {
			assert.NoError(t, fx.Flush(ctx))
			close(flushed)
		}()
		<-inSnapshot
		// Close begins while Flush holds its snapshot and has not reported yet
		closed := make(chan struct{})
		go func() {
			_ = fx.Service.Close(ctx)
			close(closed)
		}()
		require.Eventually(t, func() bool { return p.closingCtx.Err() != nil }, time.Second, time.Millisecond)
		close(proceed)
		<-flushed
		<-closed
		require.Empty(t, obs.getClosed())
	})
	t.Run("get whose ctx ends as the eviction completes does not dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			return newTestPeer(peerId), nil
		}
		for i := 0; i < 20; i++ {
			// a distinct id per round: the Get returns on its ctx while the
			// entry may still be closing
			id := fmt.Sprintf("p%d", i)
			gctx, cancel := context.WithCancel(ctx)
			dead := newCtlPeer(id)
			close(dead.closed)
			// the eviction's Close cancels the Get's ctx as it completes
			dead.closeHook = func(int32) { cancel() }
			require.NoError(t, p.current.Load().outgoing.Add(id, dead))
			_, err := fx.Get(gctx, id)
			require.ErrorIs(t, err, context.Canceled)
			cancel()
		}
		require.Zero(t, dials.Load(), "dialed with a done ctx after the eviction")
	})
	t.Run("swaps do not use up the add retries", func(t *testing.T) {
		for _, swaps := range []int{3, 5} {
			fx := newFixture(t)
			p := fx.Service.(*poolService).pool
			// every pair the pool builds already holds a connection for "d",
			// and each Pick of the duplicate triggers a flush, swaps times
			orig := p.newCaches
			p.newCaches = func() *caches {
				c := orig()
				require.NoError(t, c.incoming.Add("d", newTestPeer("d")))
				return c
			}
			require.NoError(t, fx.Flush(ctx))
			repl := newCtlPeer("d")
			var flushes atomic.Int32
			repl.idHook = func(call int32) {
				// calls 2, 4, 6...: the Pick after each ErrExists
				if call%2 == 0 && int(flushes.Load()) < swaps {
					flushes.Add(1)
					assert.NoError(t, fx.Flush(ctx))
				}
			}
			got := make(chan error, 1)
			go func() { got <- fx.AddPeer(context.Background(), repl) }()
			select {
			case err := <-got:
				require.NoError(t, err, "swaps=%d", swaps)
			case <-time.After(3 * time.Second):
				t.Fatalf("swaps=%d: AddPeer did not return", swaps)
			}
			require.Equal(t, int32(swaps), flushes.Load())
			require.True(t, inCurrent(p, repl))
			require.False(t, repl.IsClosed())
			fx.Finish()
		}
	})
	t.Run("a replacement storm for one id ends with ErrExists", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		// every eviction of the duplicate is followed by another connection
		// taking its place, as two remotes reconnecting in lockstep would
		var replaced atomic.Int32
		var inner ocache.OCache
		installIncoming(t, fx, func(in ocache.OCache, peek ocache.Peeker) ocache.OCache {
			inner = in
			return &hookedCache{OCache: in, peek: peek}
		})
		p := fx.Service.(*poolService).pool
		hc := p.current.Load().incoming.(*hookedCache)
		hc.afterRemoveSame = func() {
			// the removal has landed: a new duplicate takes the id at once
			if inner.Add("d", newTestPeer("d")) == nil {
				replaced.Add(1)
			}
		}
		require.NoError(t, inner.Add("d", newTestPeer("d")))
		repl := newCtlPeer("d")
		got := make(chan error, 1)
		go func() { got <- fx.AddPeer(context.Background(), repl) }()
		select {
		case err := <-got:
			require.ErrorIs(t, err, ocache.ErrExists)
		case <-time.After(5 * time.Second):
			t.Fatal("AddPeer did not terminate")
		}
		// the first pass and two retries each evicted one duplicate; the
		// fourth pass hit the bound before evicting another
		require.Equal(t, int32(3), replaced.Load())
	})
	t.Run("get finding a closed peer during shutdown returns ErrClosed at once", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		dead := newTestPeer("p1")
		require.NoError(t, dead.Close())
		require.NoError(t, p.current.Load().outgoing.Add("p1", dead))
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			return newTestPeer(peerId), nil
		}
		// shutdown has begun but the pair is not cancelled yet: nothing is
		// evicted any more, so the closed peer stays where it is
		p.closingCancel()
		got := make(chan error, 1)
		go func() {
			_, err := fx.Get(ctx, "p1")
			got <- err
		}()
		select {
		case err := <-got:
			require.ErrorIs(t, err, ocache.ErrClosed)
		case <-time.After(2 * time.Second):
			t.Fatal("Get spun on the closed peer")
		}
		require.Zero(t, dials.Load())
	})
	t.Run("pick leaves the eviction of a closed peer to its watcher", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// a closed peer without a watcher: nothing but a discard could close
		// or remove it
		dead := newCtlPeer("p1")
		close(dead.closed)
		require.NoError(t, p.current.Load().outgoing.Add("p1", dead))
		for i := 0; i < 10; i++ {
			_, err := fx.Pick(ctx, "p1")
			require.Error(t, err)
			require.Nil(t, p.getIfActive(ctx, []string{"p1"}))
		}
		require.Never(t, func() bool { return dead.closeCalls.Load() > 0 }, 100*time.Millisecond, 10*time.Millisecond)
		require.Equal(t, 1, p.current.Load().outgoing.Len())
	})
	t.Run("concurrent flushes report every peer exactly once before they return", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		// flush A is held inside its walk of the pair it replaced while flush
		// B replaces the pair A published and a peer is added in between
		inWalk := make(chan struct{})
		releaseWalk, doReleaseWalk := newRelease()
		defer doReleaseWalk()
		// a CAS, not sync.Once: Once would park B's walk behind A's paused one
		var armed atomic.Bool
		armed.Store(true)
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onForEach: func() {
				if armed.CompareAndSwap(true, false) {
					close(inWalk)
					<-releaseWalk
				}
			}}
		})
		x := newTestPeer("x")
		require.NoError(t, fx.AddPeer(ctx, x))
		aDone := make(chan struct{})
		go func() {
			assert.NoError(t, fx.Flush(ctx))
			close(aDone)
		}()
		<-inWalk
		// A has swapped and is walking the old pair; y lands in A's fresh pair
		y := newTestPeer("y")
		require.NoError(t, fx.AddPeer(ctx, y))
		require.NoError(t, fx.Flush(ctx))
		// B returned: y was reported by B, before A even finished its walk
		require.Equal(t, 1, len(obs.kindsFor("y")))
		require.Empty(t, obs.kindsFor("x"))
		doReleaseWalk()
		<-aDone
		require.Equal(t, 1, len(obs.kindsFor("x")))
		require.Eventually(t, func() bool { return x.IsClosed() && y.IsClosed() }, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.getClosed()) > 2 }, 200*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("a peer added while flush waits for the swap is reported before flush returns", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		x := newCtlPeer("x")
		inAdd := make(chan struct{})
		releaseAdd, doReleaseAdd := newRelease()
		defer doReleaseAdd()
		x.idHook = func(call int32) {
			if call == 1 {
				close(inAdd)
				<-releaseAdd
			}
		}
		added := make(chan error, 1)
		go func() { added <- fx.AddPeer(ctx, x) }()
		<-inAdd
		// the add holds the read lock: Flush queues behind it, so x lands in
		// the pair Flush replaces and must be in its walk
		flushed := make(chan struct{})
		go func() {
			assert.NoError(t, fx.Flush(ctx))
			close(flushed)
		}()
		time.Sleep(20 * time.Millisecond)
		doReleaseAdd()
		require.NoError(t, <-added)
		<-flushed
		require.Equal(t, []peerobserver.Kind{peerobserver.KindClosed}, obs.kindsFor("x"))
		require.Eventually(t, x.IsClosed, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.getClosed()) > 1 }, 200*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("a probe of a pair flushed under it counts no incoming miss", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			a.Register(&testMetric{reg: reg})
		})
		defer fx.Finish()
		// the pair is replaced while the Get probes its incoming cache: the
		// probe finds nothing, but that says nothing about the peer and a Get
		// on the cache would have failed uncounted
		// the first peek is the fast path's (it counts nothing itself); the
		// second is the lookup's probe, which the flush lands under
		var peeks atomic.Int32
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onPeek: func() {
				if peeks.Add(1) == 2 {
					assert.NoError(t, fx.Flush(ctx))
				}
			}}
		})
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return newTestPeer(peerId), nil
		}
		_, err := fx.Get(ctx, "out")
		require.NoError(t, err)
		families, err := reg.Gather()
		require.NoError(t, err)
		for _, mf := range families {
			if mf.GetName() == "netpool_incoming_miss" {
				// the fast path's peek and the probe of the replaced pair count
				// nothing; the retry on the fresh pair counts the one miss
				require.Equal(t, float64(1), mf.GetMetric()[0].GetCounter().GetValue())
			}
		}
	})
	t.Run("a flush landing during an eviction never dials with the dead ctx", func(t *testing.T) {
		// the swap cancels the lookup's ctx while its eviction completes; the
		// redial loop must notice before it looks again, whichever of the two
		// the select picked, or the old pair would be dialed with a dead ctx
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		var deadDials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if ctx.Err() != nil {
				deadDials.Add(1)
				return nil, ctx.Err()
			}
			return newTestPeer(peerId), nil
		}
		for i := 0; i < 40; i++ {
			id := fmt.Sprintf("p%d", i)
			dead := newCtlPeer(id)
			close(dead.closed)
			dead.closeHook = func(int32) { assert.NoError(t, fx.Flush(ctx)) }
			require.NoError(t, p.current.Load().outgoing.Add(id, dead))
			pr, err := fx.Get(ctx, id)
			require.NoError(t, err)
			require.False(t, pr.IsClosed())
		}
		require.Zero(t, deadDials.Load(), "dialed with a cancelled ctx")
	})
	t.Run("a live incoming arriving while a dead incoming is evicted is used instead of a dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// the replacement lands the moment the dead peer's removal completes,
		// before the Get looks again: it must find it instead of dialing
		live := newTestPeer("p1")
		var inner ocache.OCache
		installIncoming(t, fx, func(in ocache.OCache, peek ocache.Peeker) ocache.OCache {
			inner = in
			return &hookedCache{OCache: in, peek: peek, afterRemoveSame: func() {
				_ = in.Add("p1", live)
			}}
		})
		dead := newTestPeer("p1")
		require.NoError(t, dead.Close())
		// a dead incoming peer without a watcher
		require.NoError(t, inner.Add("p1", dead))
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			return newTestPeer(peerId), nil
		}
		pr, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		require.Same(t, live, pr)
		require.Zero(t, dials.Load(), "dialed although an incoming connection arrived")
		require.True(t, inCurrent(p, live))
	})
	t.Run("the incoming probe refreshes the deadline for Get and not for Pick", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		var mu sync.Mutex
		var touches []bool
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &peekRecorder{OCache: inner, peek: peek, record: func(touch bool) {
				mu.Lock()
				touches = append(touches, touch)
				mu.Unlock()
			}}
		})
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return newTestPeer(peerId), nil
		}
		// a dead outgoing peer keeps the fast path from settling the miss by
		// itself, so the lookup's own probe runs too
		dead := newTestPeer("out")
		require.NoError(t, dead.Close())
		require.NoError(t, fx.Service.(*poolService).pool.current.Load().outgoing.Add("out", dead))
		_, err := fx.Get(ctx, "out")
		require.NoError(t, err)
		// the fast path's peek and the lookup's probes (before and after the
		// eviction), all Get-like
		require.GreaterOrEqual(t, len(touches), 2)
		for _, touch := range touches {
			require.True(t, touch)
		}
		touches = nil
		_, err = fx.Pick(ctx, "out")
		require.NoError(t, err)
		require.Equal(t, []bool{false}, touches)
	})
	t.Run("an incoming peer found by the probe counts a hit", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			a.Register(&testMetric{reg: reg})
		})
		defer fx.Finish()
		// the peer connects between the fast path's miss and the probe (a
		// dead outgoing peer under the id keeps the fast path from settling
		// the miss by itself)
		tp := newTestPeer("in")
		var peeks atomic.Int32
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onPeek: func() {
				if peeks.Add(1) == 2 {
					assert.NoError(t, fx.AddPeer(ctx, tp))
				}
			}}
		})
		dead := newTestPeer("in")
		require.NoError(t, dead.Close())
		require.NoError(t, fx.Service.(*poolService).pool.current.Load().outgoing.Add("in", dead))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			t.Error("must not dial: an incoming connection exists")
			return nil, nil
		}
		pr, err := fx.Get(ctx, "in")
		require.NoError(t, err)
		require.Same(t, tp, pr)
		families, err := reg.Gather()
		require.NoError(t, err)
		values := map[string]float64{}
		for _, mf := range families {
			if m := mf.GetMetric()[0]; m.GetCounter() != nil {
				values[mf.GetName()] = m.GetCounter().GetValue()
			}
		}
		assert.Equal(t, float64(1), values["netpool_incoming_hit"])
		assert.Equal(t, float64(0), values["netpool_incoming_miss"])
		assert.Equal(t, float64(0), values["netpool_outgoing_hit"])
	})
	t.Run("a get whose ctx ends while a dead incoming is evicted does not dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		var deadDials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			if ctx.Err() != nil {
				deadDials.Add(1)
				return nil, ctx.Err()
			}
			return newTestPeer(peerId), nil
		}
		// the caller's ctx ends the moment the eviction's removal lands, so
		// both the discard and the ctx are ready for the select
		var cancelFn atomic.Value
		var inner ocache.OCache
		installIncoming(t, fx, func(in ocache.OCache, peek ocache.Peeker) ocache.OCache {
			inner = in
			return &hookedCache{OCache: in, peek: peek, afterRemoveSame: func() {
				cancelFn.Load().(context.CancelFunc)()
			}}
		})
		for i := 0; i < 50; i++ {
			id := fmt.Sprintf("p%d", i)
			gctx, cancel := context.WithCancel(ctx)
			cancelFn.Store(cancel)
			dead := newTestPeer(id)
			require.NoError(t, dead.Close())
			require.NoError(t, inner.Add(id, dead))
			_, err := fx.Get(gctx, id)
			require.ErrorIs(t, err, context.Canceled)
			cancel()
		}
		require.Zero(t, deadDials.Load(), "dialed with a dead ctx")
		require.Equal(t, 0, p.current.Load().outgoing.Len())
	})
	t.Run("a flush landing at the incoming probe never dials into the replaced pair", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		var dials atomic.Int32
		dialedInto := map[*caches]int{}
		var mu sync.Mutex
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			mu.Lock()
			dialedInto[p.current.Load()]++
			mu.Unlock()
			return newTestPeer(peerId), nil
		}
		// the second peek of each round is the lookup's probe: the pair is
		// replaced right there, so the probe answers ErrClosed, not a miss
		var armed atomic.Int32
		installIncoming(t, fx, func(in ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: in, peek: peek, onPeek: func() {
				if armed.Add(-1) == 0 {
					assert.NoError(t, fx.Flush(ctx))
				}
			}}
		})
		for i := 0; i < 100; i++ {
			id := fmt.Sprintf("p%d", i)
			before := dials.Load()
			armed.Store(2)
			pr, err := fx.Get(ctx, id)
			require.NoError(t, err)
			require.False(t, pr.IsClosed())
			// exactly one dial, into the pair that is current afterwards
			require.Equal(t, int32(1), dials.Load()-before)
			require.True(t, inCurrentOutgoing(p, pr))
		}
	})
	t.Run("the parallel pre-close never exceeds its cap", func(t *testing.T) {
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 300 * time.Millisecond
		})
		defer fx.Finish()
		// every peer's first close hangs; the cache's own serial pass adds at
		// most one blocked call on top of the capped workers
		var inflight, peak atomic.Int32
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		const n = 3 * maxPeerClosers
		peers := make([]*ctlPeer, 0, n)
		for i := 0; i < n; i++ {
			pr := newCtlPeer(fmt.Sprintf("in%d", i))
			pr.closeHook = func(call int32) {
				if call != 1 {
					return
				}
				cur := inflight.Add(1)
				for {
					old := peak.Load()
					if cur <= old || peak.CompareAndSwap(old, cur) {
						break
					}
				}
				<-releaseClose
				inflight.Add(-1)
			}
			peers = append(peers, pr)
			require.NoError(t, fx.AddPeer(ctx, pr))
		}
		require.NoError(t, fx.Flush(ctx))
		require.Eventually(t, func() bool { return inflight.Load() >= maxPeerClosers }, time.Second, time.Millisecond)
		require.Never(t, func() bool { return peak.Load() > maxPeerClosers+1 }, 100*time.Millisecond, time.Millisecond)
		doReleaseClose()
		require.Eventually(t, func() bool {
			for _, pr := range peers {
				if !pr.IsClosed() {
					return false
				}
			}
			return true
		}, 5*time.Second, 10*time.Millisecond)
		require.LessOrEqual(t, peak.Load(), int32(maxPeerClosers+1))
	})
	t.Run("a non-comparable peer implementation is pooled, flushed, evicted and reported", func(t *testing.T) {
		// such a peer is tracked by id instead of by instance: everything
		// works and nothing panics, as long as two connections for one id do
		// not overlap (then the id-based eviction and marking are weaker,
		// see peerKey and ocache.RemoveSame)
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// the watcher path: a dying incoming peer is evicted and reported
		a := newValuePeer("a")
		require.NoError(t, fx.AddPeer(ctx, a))
		a.close()
		// the watcher removes the entry first and reports after
		require.Eventually(t, func() bool { return p.current.Load().incoming.Len() == 0 }, time.Second, 10*time.Millisecond)
		require.Eventually(t, func() bool { return len(obs.kindsFor("a")) == 1 }, time.Second, 10*time.Millisecond)
		require.Equal(t, []peerobserver.Kind{peerobserver.KindClosed}, obs.kindsFor("a"))
		// the replacement path through RemoveSame: the old one is closed and
		// reported, the replacement survives its stale watcher
		b1, b2 := newValuePeer("b"), newValuePeer("b")
		require.NoError(t, fx.AddPeer(ctx, b1))
		require.NoError(t, fx.AddPeer(ctx, b2))
		require.True(t, b1.IsClosed())
		require.Eventually(t, func() bool { return len(obs.kindsFor("b")) == 1 }, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return b2.IsClosed() || p.current.Load().incoming.Len() != 1 }, 100*time.Millisecond, 10*time.Millisecond)
		b2.close()
		require.Eventually(t, func() bool { return p.current.Load().incoming.Len() == 0 && len(obs.kindsFor("b")) == 2 }, time.Second, 10*time.Millisecond)
		// the discard path: a dead outgoing peer found by a lookup (added
		// without a watcher, stored as the pool would store it)
		dead := newValuePeer("c")
		dead.close()
		require.NoError(t, p.current.Load().outgoing.Add("c", wrap(dead)))
		fresh := newValuePeer("c")
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return fresh, nil
		}
		pr, err := fx.Get(ctx, "c")
		require.NoError(t, err)
		require.False(t, pr.IsClosed())
		require.Equal(t, 1, p.current.Load().outgoing.Len())
		// the flush path: the remaining peer is reported before Flush returns
		require.NoError(t, fx.Flush(ctx))
		require.Equal(t, []peerobserver.Kind{peerobserver.KindClosed}, obs.kindsFor("c"))
		require.Eventually(t, fresh.IsClosed, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.getClosed()) > 4 }, 200*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("a non-comparable peer's stale watcher never touches its replacement and flush marks never hide it", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// the old instance's removal is held while the replacement lands and
		// a flush marks the replacement: the stale watcher must neither close
		// the replacement nor be silenced by the mark on it
		// a CAS gate, not sync.Once: the replacement's own removal goes
		// through the same seam and must not queue behind the held one
		var armed atomic.Bool
		armed.Store(true)
		inRemove := make(chan struct{})
		releaseRemove, doReleaseRemove := newRelease()
		defer doReleaseRemove()
		installIncoming(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onRemoveSame: func() {
				if armed.CompareAndSwap(true, false) {
					close(inRemove)
					<-releaseRemove
				}
			}}
		})
		a := newValuePeer("x")
		require.NoError(t, fx.AddPeer(ctx, a))
		a.close()
		// a's watcher is inside its RemoveSame now
		<-inRemove
		b := newValuePeer("x")
		require.NoError(t, fx.AddPeer(ctx, b))
		require.NoError(t, fx.Flush(ctx))
		// Flush reported b (and closes it); a's watcher still owes a's event
		require.Len(t, obs.kindsFor("x"), 1)
		doReleaseRemove()
		require.Eventually(t, func() bool { return len(obs.kindsFor("x")) == 2 }, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.kindsFor("x")) > 2 }, 200*time.Millisecond, 10*time.Millisecond)

		// and without a flush: the stale watcher's removal leaves the live
		// replacement in place
		c1 := newValuePeer("y")
		require.NoError(t, fx.AddPeer(ctx, c1))
		c2 := newValuePeer("y")
		require.NoError(t, fx.AddPeer(ctx, c2))
		require.True(t, c1.IsClosed())
		require.Eventually(t, func() bool { return len(obs.kindsFor("y")) == 1 }, time.Second, 10*time.Millisecond)
		require.False(t, c2.IsClosed())
		v, err := p.current.Load().incoming.Pick(ctx, "y")
		require.NoError(t, err)
		got, err := getPeer(v)
		require.NoError(t, err)
		require.Equal(t, "y", got.Id())
		require.False(t, got.IsClosed())
	})
	t.Run("a lookup parked in an eviction when the pool closes gets ErrClosed", func(t *testing.T) {
		// the pair stays current but is cancelled by Close: the ctx error the
		// eviction's select returns must surface as ErrClosed
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 200 * time.Millisecond
		})
		p := fx.Service.(*poolService).pool
		dead := newCtlPeer("p1")
		close(dead.closed)
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		dead.closeHook = func(int32) { <-releaseClose }
		require.NoError(t, p.current.Load().outgoing.Add("p1", dead))
		got := make(chan error, 1)
		go func() {
			_, err := fx.Get(ctx, "p1")
			got <- err
		}()
		require.Eventually(t, func() bool { return dead.closeCalls.Load() == 1 }, time.Second, time.Millisecond)
		fx.Finish()
		require.ErrorIs(t, <-got, ocache.ErrClosed)
	})
	t.Run("an incompatible-version verdict from the dialer survives flush", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			return nil, handshake.ErrIncompatibleVersion
		}
		_, err := fx.Get(ctx, "p1")
		require.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
		require.NoError(t, fx.Flush(ctx))
		_, err = fx.Get(ctx, "p1")
		require.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
		_, err = fx.Pick(ctx, "p1")
		require.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
		require.Equal(t, int32(1), dials.Load(), "the verdict was redialed after the flush")
	})
	t.Run("a dial completing after flush is evicted from the pair it loaded into", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		// each pair's outgoing cache records the ids RemoveSame is called for
		var mu sync.Mutex
		removed := map[*hookedCache][]string{}
		var hooks []*hookedCache
		installOutgoing(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			hc := &hookedCache{OCache: inner, peek: peek}
			hc.onRemoveSameID = func(id string) {
				mu.Lock()
				removed[hc] = append(removed[hc], id)
				mu.Unlock()
			}
			mu.Lock()
			hooks = append(hooks, hc)
			mu.Unlock()
			return hc
		})
		oldPair := p.current.Load()
		oldHook := oldPair.outgoing.(*hookedCache)
		late := newTestPeer("p1")
		dialStarted := make(chan struct{})
		releaseDial, doReleaseDial := newRelease()
		defer doReleaseDial()
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			close(dialStarted)
			<-releaseDial // completes despite the cancellation
			return late, nil
		}
		loaded := make(chan struct{})
		go func() {
			_, _ = oldPair.outgoing.Get(ctx, "p1")
			close(loaded)
		}()
		<-dialStarted
		require.NoError(t, fx.Flush(ctx))
		newHook := p.current.Load().outgoing.(*hookedCache)
		doReleaseDial()
		<-loaded
		// the late peer is closed by the old pair; its watcher evicts it from
		// the old pair, not from the one that is current now
		require.Eventually(t, late.IsClosed, time.Second, 10*time.Millisecond)
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(removed[oldHook]) == 1
		}, time.Second, 10*time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, []string{"p1"}, removed[oldHook])
		require.Empty(t, removed[newHook])
	})
	t.Run("a GC TryClose the incoming peer declines does not cause a dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		in := newCtlPeer("p1")
		inTryClose := make(chan struct{})
		releaseTryClose, doReleaseTryClose := newRelease()
		defer doReleaseTryClose()
		in.tryClose = func() (bool, error) {
			close(inTryClose)
			<-releaseTryClose
			return false, nil // in use: stays
		}
		require.NoError(t, fx.AddPeer(ctx, in))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			t.Error("dialed although an incoming connection exists")
			return nil, nil
		}
		// the GC holds the entry in closing while TryClose runs
		gcDone := make(chan struct{})
		go func() {
			defer close(gcDone)
			_, _ = p.current.Load().incoming.TryRemove("p1")
		}()
		<-inTryClose
		got := make(chan peer.Peer, 1)
		go func() {
			pr, err := fx.Get(ctx, "p1")
			assert.NoError(t, err)
			got <- pr
		}()
		// the Get waits for the close to resolve instead of dialing
		require.Never(t, func() bool { return len(got) > 0 }, 50*time.Millisecond, 5*time.Millisecond)
		doReleaseTryClose()
		<-gcDone
		require.Same(t, in, <-got)
	})
	t.Run("the loader never dials for a replaced pair", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		var dials atomic.Int32
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			dials.Add(1)
			return newTestPeer(peerId), nil
		}
		// the pair is replaced as the outgoing load starts, before the loader
		// runs: no dial for the old pair, one for the fresh one
		var armed atomic.Bool
		installOutgoing(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onGet: func() {
				if armed.CompareAndSwap(true, false) {
					assert.NoError(t, fx.Flush(ctx))
				}
			}}
		})
		armed.Store(true)
		pr, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		require.False(t, pr.IsClosed())
		require.Equal(t, int32(1), dials.Load())
	})
	t.Run("a dial in flight ends with its pair even for a direct load", func(t *testing.T) {
		// the dial ctx is bound to the pair by the loader itself, not only
		// through the lookup's ctx or through the cache's Close (which
		// cancels loads too, but is held back here)
		fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
			ps.closeTimeout = 2 * time.Second
		})
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		var armed atomic.Bool
		installOutgoing(t, fx, func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache {
			return &hookedCache{OCache: inner, peek: peek, onClose: func() {
				if armed.CompareAndSwap(true, false) {
					<-releaseClose
				}
			}}
		})
		dialStarted := make(chan struct{})
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			close(dialStarted)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		oldPair := p.current.Load()
		loaded := make(chan error, 1)
		go func() {
			_, err := oldPair.outgoing.Get(ctx, "p1")
			loaded <- err
		}()
		<-dialStarted
		// the old pair's outgoing Close is held: only the pair ctx can end
		// the dial now
		armed.Store(true)
		require.NoError(t, fx.Flush(ctx))
		select {
		case err := <-loaded:
			require.Error(t, err)
		case <-time.After(time.Second):
			t.Fatal("the dial outlived its pair")
		}
	})
	t.Run("a peer comparable by type but not by value is tracked without panicking", func(t *testing.T) {
		// a struct with an interface field: the type is comparable, the value
		// is not once the field holds a slice; == and a map insert panic
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		a := newIfacePeer("a")
		require.NoError(t, fx.AddPeer(ctx, a))
		a.close()
		require.Eventually(t, func() bool { return p.current.Load().incoming.Len() == 0 }, time.Second, 10*time.Millisecond)
		require.Eventually(t, func() bool { return len(obs.kindsFor("a")) == 1 }, time.Second, 10*time.Millisecond)
		b := newIfacePeer("b")
		require.NoError(t, fx.AddPeer(ctx, b))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return newIfacePeer(peerId), nil
		}
		_, err := fx.Get(ctx, "c")
		require.NoError(t, err)
		require.NoError(t, fx.Flush(ctx))
		require.Equal(t, []peerobserver.Kind{peerobserver.KindClosed}, obs.kindsFor("b"))
		require.Equal(t, []peerobserver.Kind{peerobserver.KindClosed}, obs.kindsFor("c"))
		require.Eventually(t, b.IsClosed, time.Second, 10*time.Millisecond)
		require.Never(t, func() bool { return len(obs.getClosed()) > 3 }, 200*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("a miss on both caches allocates nothing for Pick and GetOneOf's scan", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool
		require.Zero(t, testing.AllocsPerRun(100, func() {
			if _, err := fx.Pick(ctx, "absent"); err == nil {
				t.Error("unexpected hit")
			}
		}))
		ids := []string{"absent1", "absent2"}
		require.Zero(t, testing.AllocsPerRun(100, func() {
			if p.getIfActive(ctx, ids) != nil {
				t.Error("unexpected hit")
			}
		}))
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
	deadline := time.Now().Add(15 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines before=%d after=%d", before, runtime.NumGoroutine())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// kindsFor returns the kinds of every event recorded for peerId, in order
func (r *poolEventRecorder) kindsFor(peerId string) (kinds []peerobserver.Kind) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ev := range r.events {
		if ev.PeerId == peerId {
			kinds = append(kinds, ev.Kind)
		}
	}
	return
}

// TestPool_FastPathMetrics pins the series of the hit path against what a Get
// or Pick through the caches counts: exactly once per cache per call, also
// when the fast path finds a closed peer and falls through to lookup
func TestPool_FastPathMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	fx := newFixtureCfg(t, nil, func(ps *poolService, a *app.App) {
		a.Register(&testMetric{reg: reg})
	})
	defer fx.Finish()
	p := fx.Service.(*poolService).pool
	fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
		return newTestPeer(peerId), nil
	}
	counters := func() map[string]float64 {
		families, err := reg.Gather()
		require.NoError(t, err)
		out := map[string]float64{}
		for _, mf := range families {
			if m := mf.GetMetric()[0]; m.GetCounter() != nil {
				out[mf.GetName()] = m.GetCounter().GetValue()
			}
		}
		return out
	}
	expect := func(step string, incomingHit, incomingMiss, outgoingHit, outgoingMiss float64) {
		got := counters()
		assert.Equal(t, map[string]float64{
			"netpool_incoming_hit": incomingHit, "netpool_incoming_miss": incomingMiss, "netpool_incoming_gc": 0,
			"netpool_outgoing_hit": outgoingHit, "netpool_outgoing_miss": outgoingMiss, "netpool_outgoing_gc": 0,
		}, got, step)
	}
	_, err := fx.Get(ctx, "out") // incoming miss, outgoing miss (dial)
	require.NoError(t, err)
	expect("dial", 0, 1, 0, 1)
	_, err = fx.Get(ctx, "out") // fast: incoming miss, outgoing hit
	require.NoError(t, err)
	expect("fast get", 0, 2, 1, 1)
	_, err = fx.Pick(ctx, "out") // fast: Pick counts the hit only
	require.NoError(t, err)
	expect("fast pick", 0, 2, 2, 1)
	_, err = fx.GetOneOf(ctx, []string{"zz", "out"}) // a Pick miss counts nothing, then the hit
	require.NoError(t, err)
	expect("fast getoneof", 0, 2, 3, 1)

	// a closed peer in the cache: fast counts nothing and lookup takes over
	// (incoming miss, outgoing hit on the closed entry, then after the
	// eviction incoming miss and outgoing miss for the dial)
	dead := newTestPeer("d")
	require.NoError(t, dead.Close())
	require.NoError(t, p.current.Load().outgoing.Add("d", dead))
	_, err = fx.Get(ctx, "d")
	require.NoError(t, err)
	expect("fallback", 0, 4, 4, 2)

	require.NoError(t, fx.AddPeer(ctx, newTestPeer("in")))
	_, err = fx.Get(ctx, "in") // fast: incoming hit
	require.NoError(t, err)
	_, err = fx.Pick(ctx, "in")
	require.NoError(t, err)
	expect("incoming", 2, 4, 4, 2)
}

// stickyCache is an OCache whose RemoveSame never removes anything
type stickyCache struct {
	ocache.OCache
	peek ocache.Peeker
}

func (c *stickyCache) RemoveSame(ctx context.Context, id string, value ocache.Object) (bool, error) {
	return false, nil
}

func (c *stickyCache) Peek(id string, touch bool) (ocache.Object, ocache.PeekState) {
	return c.peek.Peek(id, touch)
}

func (c *stickyCache) WaitClosing(ctx context.Context, id string) error {
	return c.peek.WaitClosing(ctx, id)
}

// hookedCache is an incoming cache whose RemoveSame and ForEach can be
// intercepted: the seams for racing a Flush against a watcher's eviction
type hookedCache struct {
	ocache.OCache
	peek            ocache.Peeker
	onRemoveSame    func()
	onRemoveSameID  func(id string)
	afterRemoveSame func()
	onForEach       func()
	onGet           func()
	onPeek          func()
	onClose         func()
}

func (c *hookedCache) RemoveSame(ctx context.Context, id string, value ocache.Object) (bool, error) {
	if c.onRemoveSame != nil {
		c.onRemoveSame()
	}
	if c.onRemoveSameID != nil {
		c.onRemoveSameID(id)
	}
	ok, err := c.OCache.RemoveSame(ctx, id, value)
	if c.afterRemoveSame != nil {
		c.afterRemoveSame()
	}
	return ok, err
}

func (c *hookedCache) Get(ctx context.Context, id string) (ocache.Object, error) {
	if c.onGet != nil {
		c.onGet()
	}
	return c.OCache.Get(ctx, id)
}

func (c *hookedCache) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return c.OCache.Close()
}

func (c *hookedCache) ForEach(f func(v ocache.Object) bool) {
	c.OCache.ForEach(f)
	if c.onForEach != nil {
		c.onForEach()
	}
}

func (c *hookedCache) Peek(id string, touch bool) (ocache.Object, ocache.PeekState) {
	if c.onPeek != nil {
		c.onPeek()
	}
	return c.peek.Peek(id, touch)
}

func (c *hookedCache) WaitClosing(ctx context.Context, id string) error {
	return c.peek.WaitClosing(ctx, id)
}

// installOutgoing is installIncoming for the outgoing cache
func installOutgoing(t *testing.T, fx *fixture, wrap func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache) {
	p := fx.Service.(*poolService).pool
	orig := p.newCaches
	p.newCaches = func() *caches {
		c := orig()
		c.outgoing = wrap(c.outgoing, c.peekOutgoing)
		c.peekOutgoing = mustPeeker(c.outgoing)
		return c
	}
	require.NoError(t, fx.Flush(ctx))
}

// installIncoming makes every pair the pool builds from now on use wrap(inner)
// as its incoming cache, and flushes once so the current pair has it
func installIncoming(t *testing.T, fx *fixture, wrap func(inner ocache.OCache, peek ocache.Peeker) ocache.OCache) {
	p := fx.Service.(*poolService).pool
	orig := p.newCaches
	p.newCaches = func() *caches {
		c := orig()
		c.incoming = wrap(c.incoming, c.peekIncoming)
		c.peekIncoming = mustPeeker(c.incoming)
		return c
	}
	require.NoError(t, fx.Flush(ctx))
}

// peekRecorder is an incoming cache that records the touch flag of every Peek
type peekRecorder struct {
	ocache.OCache
	peek   ocache.Peeker
	record func(touch bool)
}

func (c *peekRecorder) Peek(id string, touch bool) (ocache.Object, ocache.PeekState) {
	c.record(touch)
	return c.peek.Peek(id, touch)
}

func (c *peekRecorder) WaitClosing(ctx context.Context, id string) error {
	return c.peek.WaitClosing(ctx, id)
}

func inCurrentOutgoing(p *pool, pr peer.Peer) bool {
	v, err := p.current.Load().outgoing.Pick(ctx, pr.Id())
	return err == nil && v == ocache.Object(pr)
}

// valuePeer is a peer.Peer implementation that is not comparable (a slice
// field, value receivers): == on two of them panics, so the pool must never
// compare peers directly
type valuePeer struct {
	id     string
	mu     *sync.Mutex
	closed chan struct{}
	tags   []string
}

func newValuePeer(id string) valuePeer {
	return valuePeer{id: id, mu: &sync.Mutex{}, closed: make(chan struct{}), tags: []string{id}}
}

// close is idempotent under concurrent callers (the pool closes a flushed
// peer from the pre-close and from the cache's pass)
func (v valuePeer) close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	select {
	case <-v.closed:
	default:
		close(v.closed)
	}
}

func (v valuePeer) Id() string                           { return v.id }
func (v valuePeer) Addr() string                         { return "" }
func (v valuePeer) Close() error                         { v.close(); return nil }
func (v valuePeer) TryClose(time.Duration) (bool, error) { v.close(); return true, nil }
func (v valuePeer) IsClosed() bool {
	select {
	case <-v.closed:
		return true
	default:
		return false
	}
}
func (v valuePeer) CloseChan() <-chan struct{} { return v.closed }
func (v valuePeer) SetTTL(time.Duration)       {}
func (v valuePeer) DoDrpc(context.Context, func(conn drpc.Conn) error) error {
	return fmt.Errorf("not implemented")
}
func (v valuePeer) AcquireDrpcConn(context.Context) (drpc.Conn, error) {
	return nil, fmt.Errorf("not implemented")
}
func (v valuePeer) ReleaseDrpcConn(context.Context, drpc.Conn) {}
func (v valuePeer) Context() context.Context                   { return ctx }
func (v valuePeer) Accept() (net2.Conn, error)                 { return nil, fmt.Errorf("not implemented") }
func (v valuePeer) Open(context.Context) (net2.Conn, error) {
	return nil, fmt.Errorf("not implemented")
}

var _ peer.Peer = valuePeer{}

// ifacePeer is comparable as a type (no slice or map fields of its own) but
// not as a value: its payload field holds a slice, so == on two of them, or a
// map insert, panics. Value receivers, like valuePeer.
type ifacePeer struct {
	id      string
	mu      *sync.Mutex
	closed  chan struct{}
	payload any
}

func newIfacePeer(id string) ifacePeer {
	return ifacePeer{id: id, mu: &sync.Mutex{}, closed: make(chan struct{}), payload: []string{id}}
}

func (v ifacePeer) close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	select {
	case <-v.closed:
	default:
		close(v.closed)
	}
}

func (v ifacePeer) Id() string                           { return v.id }
func (v ifacePeer) Addr() string                         { return "" }
func (v ifacePeer) Close() error                         { v.close(); return nil }
func (v ifacePeer) TryClose(time.Duration) (bool, error) { v.close(); return true, nil }
func (v ifacePeer) IsClosed() bool {
	select {
	case <-v.closed:
		return true
	default:
		return false
	}
}
func (v ifacePeer) CloseChan() <-chan struct{} { return v.closed }
func (v ifacePeer) SetTTL(time.Duration)       {}
func (v ifacePeer) DoDrpc(context.Context, func(conn drpc.Conn) error) error {
	return fmt.Errorf("not implemented")
}
func (v ifacePeer) AcquireDrpcConn(context.Context) (drpc.Conn, error) {
	return nil, fmt.Errorf("not implemented")
}
func (v ifacePeer) ReleaseDrpcConn(context.Context, drpc.Conn) {}
func (v ifacePeer) Context() context.Context                   { return ctx }
func (v ifacePeer) Accept() (net2.Conn, error)                 { return nil, fmt.Errorf("not implemented") }
func (v ifacePeer) Open(context.Context) (net2.Conn, error) {
	return nil, fmt.Errorf("not implemented")
}

var _ peer.Peer = ifacePeer{}
