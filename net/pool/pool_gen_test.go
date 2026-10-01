package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peerobserver"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
)

// ctlPeer is a test peer whose TryClose and Close can be paused or made to
// decline, to drive the races between Flush, the ocache GC and the lookups
type ctlPeer struct {
	*testPeer
	tryClose   func() (bool, error)
	closeHook  func(call int32)
	closeCalls atomic.Int32
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

var _ peer.Peer = (*ctlPeer)(nil)

// newRelease returns a gate channel and an idempotent func that opens it
func newRelease() (chan struct{}, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return ch, func() { once.Do(func() { close(ch) }) }
}

func TestPool_FlushGeneration(t *testing.T) {
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

		// the GC path: the entry is held in closing while TryClose runs
		gcDone := make(chan struct{})
		go func() {
			defer close(gcDone)
			_, _ = p.outgoing.TryRemove("p1")
		}()
		<-inTryClose

		// the removal pass cannot see the closing entry, the generation does
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

		// the declined peer is active in the cache again, but stale
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
	})
	t.Run("late published dial is rejected on first lookup and closed", func(t *testing.T) {
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
			<-releaseDial
			return late, nil
		}
		loaded := make(chan struct{})
		go func() {
			defer close(loaded)
			// a background loader whose result nobody looks at
			_, _ = p.outgoing.Get(ctx, "p1")
		}()
		<-dialStarted
		require.NoError(t, fx.Flush(ctx))
		doReleaseDial()
		<-loaded

		// published after the flush, but stamped with the pre-flush
		// generation: closed without any lookup
		require.Eventually(t, late.IsClosed, time.Second, 10*time.Millisecond)
		_, err := fx.Pick(ctx, "p1")
		require.Error(t, err)
		require.Eventually(t, func() bool { return p.outgoing.Len() == 0 }, time.Second, 10*time.Millisecond)
		// the rejected peer still gets exactly one Closed event
		require.Eventually(t, func() bool { return len(obs.getClosed()) == 1 }, time.Second, 10*time.Millisecond)
		assert.False(t, obs.getClosed()[0].Inbound)
		require.Never(t, func() bool { return len(obs.getClosed()) > 1 }, 100*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("stale peer held by a declined TryClose is closed without any lookup", func(t *testing.T) {
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

		gcDone := make(chan struct{})
		go func() {
			defer close(gcDone)
			_, _ = p.outgoing.TryRemove("p1")
		}()
		<-inTryClose
		require.NoError(t, fx.Flush(ctx))
		// the cache does not list the closing entry, the stamps do
		require.Eventually(t, old.IsClosed, time.Second, 10*time.Millisecond)
		doReleaseTryClose()
		<-gcDone
		require.Eventually(t, func() bool { return p.outgoing.Len() == 0 }, time.Second, 10*time.Millisecond)
	})
	t.Run("concurrent waiters on a load spanning flush redial once", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()

		late := newTestPeer("p1")
		fresh := newTestPeer("p1")
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
			return fresh, nil
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
		for i := 0; i < waiters; i++ {
			select {
			case pr := <-results:
				assert.True(t, pr == peer.Peer(fresh), "a stale peer was returned")
			case <-time.After(5 * time.Second):
				t.Fatal("waiter did not return")
			}
		}
		assert.Equal(t, int32(2), dials.Load(), "one redial for all waiters")
		require.Eventually(t, late.IsClosed, time.Second, 10*time.Millisecond)
		assert.False(t, fresh.IsClosed())
	})
	t.Run("get on a stale peer whose close blocks returns within ctx", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()

		old := newCtlPeer("p1")
		releaseClose, doReleaseClose := newRelease()
		defer doReleaseClose()
		old.closeHook = func(int32) { <-releaseClose }
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return old, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		require.NoError(t, fx.Flush(ctx))

		// the stale peer's teardown hangs: Get must not run or wait on it
		// past its own deadline
		done := make(chan error, 1)
		go func() {
			gctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()
			_, gErr := fx.Get(gctx, "p1")
			done <- gErr
		}()
		select {
		case err = <-done:
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
				_ = fx.AddPeer(ctx, tp)
			}()
			go func() {
				defer wg.Done()
				_ = fx.Flush(ctx)
			}()
			wg.Wait()
			// with no lookup at all, the peer is either current or closed
			require.Eventually(t, func() bool { return tp.IsClosed() || !p.isStale(tp) }, time.Second, time.Millisecond)
			pr, err := fx.Pick(ctx, id)
			if err == nil {
				// served only while it belongs to the current generation
				require.Equal(t, peer.Peer(tp), pr)
				require.False(t, p.isStale(tp))
				require.False(t, tp.IsClosed())
			} else {
				// stamped before the bump: closed by the flush
				require.Eventually(t, tp.IsClosed, time.Second, time.Millisecond)
			}
		}
	})
	t.Run("flush keeps a current-generation peer", func(t *testing.T) {
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

		require.NoError(t, p.outgoing.Add("p1", &errObject{id: "p1", gen: p.gen.Load(), err: handshake.ErrIncompatibleVersion}))
		require.NoError(t, fx.Flush(ctx))
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			t.Fatal("must not redial an incompatible peer")
			return nil, nil
		}
		_, err := fx.Get(ctx, "p1")
		require.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
		_, err = fx.Pick(ctx, "p1")
		require.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
	})
	t.Run("cached dial error is dropped by flush", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		require.NoError(t, p.outgoing.Add("p1", &errObject{id: "p1", gen: p.gen.Load(), err: assert.AnError}))
		_, err := fx.Pick(ctx, "p1")
		require.ErrorIs(t, err, assert.AnError)

		require.NoError(t, fx.Flush(ctx))
		fresh := newTestPeer("p1")
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer.Peer, error) {
			return fresh, nil
		}
		pr, err := fx.Get(ctx, "p1")
		require.NoError(t, err)
		assert.Equal(t, peer.Peer(fresh), pr)
	})
	t.Run("dial error cached across flush is not served", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

		// published after the flush with the pre-flush generation
		staleGen := p.gen.Load()
		require.NoError(t, fx.Flush(ctx))
		require.NoError(t, p.outgoing.Add("p1", &errObject{id: "p1", gen: staleGen, err: assert.AnError}))

		_, err := fx.Pick(ctx, "p1")
		require.Error(t, err)
		require.NotErrorIs(t, err, assert.AnError)
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

		// the peer reconnects: the new incoming peer belongs to the current
		// generation
		repl := newTestPeer("p1")
		require.Eventually(t, func() bool { return fx.AddPeer(ctx, repl) == nil }, time.Second, 10*time.Millisecond)
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
			// the flush's async close is the first one: hold it, so its
			// RemoveSame runs only after the replacement is installed
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
		require.Equal(t, 1, p.incoming.Len())
	})
	t.Run("repeated flush", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p := fx.Service.(*poolService).pool

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
		require.Equal(t, uint64(6), p.gen.Load())
		for _, tp := range peers {
			require.Eventually(t, tp.IsClosed, time.Second, 10*time.Millisecond)
		}
		require.Eventually(t, func() bool {
			p.stampsMu.Lock()
			defer p.stampsMu.Unlock()
			return len(p.stamps) == 0
		}, time.Second, 10*time.Millisecond)
	})
	t.Run("flush then shutdown", func(t *testing.T) {
		obs := &poolEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
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
		// a flush on a closed pool is a no-op
		require.NoError(t, fx.Flush(ctx))
		_, err = fx.Pick(ctx, "in")
		require.Error(t, err)
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
