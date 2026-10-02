package peer

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"storj.io/drpc"
	"storj.io/drpc/drpcwire"

	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
)

type rawMsg []byte

type rawEncoding struct{}

func (rawEncoding) Marshal(msg drpc.Message) ([]byte, error) { return *msg.(*rawMsg), nil }

func (rawEncoding) Unmarshal(buf []byte, msg drpc.Message) error {
	*msg.(*rawMsg) = append([]byte(nil), buf...)
	return nil
}

// blockingCloser is a closer whose Close blocks until released
type blockingCloser struct {
	release chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockingCloser(release chan struct{}) *blockingCloser {
	return &blockingCloser{release: release, closed: make(chan struct{})}
}

func (b *blockingCloser) Close() error {
	<-b.release
	b.once.Do(func() { close(b.closed) })
	return nil
}

// blockingCloseConn models a stalled sub connection whose Close blocks on
// the transport until released
type blockingCloseConn struct {
	net.Conn
	release chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockingCloseConn(conn net.Conn, release chan struct{}) *blockingCloseConn {
	return &blockingCloseConn{Conn: conn, release: release, closed: make(chan struct{})}
}

func (c *blockingCloseConn) Close() error {
	<-c.release
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

// quickCloser models a healthy sub conn close: about one round trip
type quickCloser struct {
	d       time.Duration
	running *atomic.Int32
	maxSeen *atomic.Int32
	closed  chan struct{}
}

func (q *quickCloser) Close() error {
	n := q.running.Add(1)
	for {
		m := q.maxSeen.Load()
		if n <= m || q.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(q.d)
	q.running.Add(-1)
	close(q.closed)
	return nil
}

func TestCleanupOwner(t *testing.T) {
	t.Run("burst never blocks the caller and never drops a close", func(t *testing.T) {
		c := newCleanupOwner("p1")

		const burst = 300
		var running, maxSeen atomic.Int32
		closers := make([]*quickCloser, burst)
		var wg sync.WaitGroup
		for i := range closers {
			closers[i] = &quickCloser{d: 2 * time.Millisecond, running: &running, maxSeen: &maxSeen, closed: make(chan struct{})}
		}
		start := time.Now()
		for _, cl := range closers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.close(cl)
			}()
		}
		wg.Wait()
		assert.Less(t, time.Since(start), 500*time.Millisecond, "close must never block")
		for _, cl := range closers {
			select {
			case <-cl.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("a close was dropped")
			}
		}
		assert.LessOrEqual(t, int(maxSeen.Load()), cleanupMaxWorkers)
		// workers exit once idle: an idle peer costs no goroutines
		require.Eventually(t, func() bool {
			r, p := c.stats()
			return r == 0 && p == 0
		}, time.Second, time.Millisecond)
	})
	t.Run("saturated workers queue and drain", func(t *testing.T) {
		c := newCleanupOwner("p1")
		release := make(chan struct{})
		var releaseOnce sync.Once
		doRelease := func() { releaseOnce.Do(func() { close(release) }) }
		defer doRelease()

		var closers []*blockingCloser
		for i := 0; i < cleanupMaxWorkers+5; i++ {
			cl := newBlockingCloser(release)
			closers = append(closers, cl)
			start := time.Now()
			c.close(cl)
			require.Less(t, time.Since(start), 100*time.Millisecond, "close must never block")
		}
		r, p := c.stats()
		assert.Equal(t, cleanupMaxWorkers, r)
		assert.Equal(t, 5, p)
		assert.Equal(t, cleanupMaxWorkers+5, c.inFlight())

		doRelease()
		for _, cl := range closers {
			select {
			case <-cl.closed:
			case <-time.After(time.Second):
				t.Fatal("a queued close was dropped")
			}
		}
		require.Eventually(t, func() bool {
			r, p := c.stats()
			return r == 0 && p == 0 && c.inFlight() == 0
		}, time.Second, time.Millisecond)
	})
	t.Run("a hung close blocks neither the caller nor other closes", func(t *testing.T) {
		c := newCleanupOwner("p1")
		hang := make(chan struct{})
		defer close(hang)
		hung := newBlockingCloser(hang)
		start := time.Now()
		c.close(hung)
		require.Less(t, time.Since(start), 100*time.Millisecond)

		var running, maxSeen atomic.Int32
		var closers []*quickCloser
		for i := 0; i < 200; i++ {
			cl := &quickCloser{d: time.Millisecond, running: &running, maxSeen: &maxSeen, closed: make(chan struct{})}
			closers = append(closers, cl)
			c.close(cl)
		}
		for _, cl := range closers {
			select {
			case <-cl.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("a close was held up by the hung one")
			}
		}
		// only the hung close is left, on its own worker
		require.Eventually(t, func() bool {
			r, p := c.stats()
			return r == 1 && p == 0 && c.inFlight() == 1
		}, time.Second, time.Millisecond)
	})
	t.Run("sustained close rate", func(t *testing.T) {
		c := newCleanupOwner("p1")

		// far more closes than workers, arriving faster than they finish
		const total = 3000
		var running, maxSeen atomic.Int32
		closers := make([]*quickCloser, total)
		for i := range closers {
			closers[i] = &quickCloser{d: 5 * time.Millisecond, running: &running, maxSeen: &maxSeen, closed: make(chan struct{})}
		}
		var sawInFlight int
		for i, cl := range closers {
			c.close(cl)
			if i%100 == 0 {
				time.Sleep(time.Millisecond)
				sawInFlight = max(sawInFlight, c.inFlight())
			}
		}
		for _, cl := range closers {
			select {
			case <-cl.closed:
			case <-time.After(10 * time.Second):
				t.Fatal("a close was dropped")
			}
		}
		assert.LessOrEqual(t, int(maxSeen.Load()), cleanupMaxWorkers)
		// closes in flight are visible to the peer's open limiter
		assert.Greater(t, sawInFlight, cleanupMaxWorkers)
		require.Eventually(t, func() bool { return c.inFlight() == 0 }, time.Second, time.Millisecond)
	})
	t.Run("slow close is logged once", func(t *testing.T) {
		c := newCleanupOwner("p1")
		c.slowClose = 20 * time.Millisecond
		var logged atomic.Int32
		c.onSlowClose = func(io.Closer) { logged.Add(1) }
		release := make(chan struct{})
		cl := newBlockingCloser(release)
		c.close(cl)
		require.Eventually(t, func() bool { return logged.Load() == 1 }, time.Second, time.Millisecond)
		require.Never(t, func() bool { return logged.Load() > 1 }, 100*time.Millisecond, 10*time.Millisecond)
		close(release)
		<-cl.closed
		// a quick close is not logged
		quick := newBlockingCloser(release)
		c.close(quick)
		<-quick.closed
		require.Never(t, func() bool { return logged.Load() > 1 }, 50*time.Millisecond, 10*time.Millisecond)
	})
	t.Run("nil owner", func(t *testing.T) {
		var c *cleanupOwner
		release := make(chan struct{})
		cl := newBlockingCloser(release)
		returnsWithin(t, 100*time.Millisecond, "close must never block", func() { c.close(cl) })
		close(release)
		select {
		case <-cl.closed:
		case <-time.After(time.Second):
			t.Fatal("close was dropped")
		}
	})
}

func TestPeer_HandshakeFailureCloseDoesNotBlock(t *testing.T) {
	t.Run("protocol error", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		release := make(chan struct{})
		defer close(release)

		in, out := net.Pipe()
		defer out.Close()
		conn := newBlockingCloseConn(in, release)
		// the remote declines the protocol, which leaves the close to openDrpcConn
		go func() {
			_, _ = handshake.IncomingProtoHandshake(ctx, out, handshake.ProtoChecker{
				AllowedProtoTypes: []handshakeproto.ProtoType{handshakeproto.ProtoType(100)},
			})
		}()
		fx.mc.EXPECT().Open(gomock.Any()).Return(conn, nil)

		start := time.Now()
		_, err := fx.AcquireDrpcConn(ctx)
		require.ErrorIs(t, err, handshake.ErrRemoteIncompatibleProto)
		assert.Less(t, time.Since(start), time.Second)
	})
	t.Run("deadline", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		release := make(chan struct{})

		in, out := net.Pipe()
		defer out.Close()
		conn := newBlockingCloseConn(in, release)
		// the remote never answers
		go func() { _, _ = io.Copy(io.Discard, out) }()
		fx.mc.EXPECT().Open(gomock.Any()).Return(conn, nil)

		actx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := fx.AcquireDrpcConn(actx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), time.Second)

		// the stream is still closed once the transport lets it
		close(release)
		select {
		case <-conn.closed:
		case <-time.After(time.Second):
			t.Fatal("stream was not closed")
		}
	})
}

// TestPeer_RPCDeadlineWithBlockedClose is the payment-call shape: an
// established RPC hits its deadline on a stalled connection whose stream
// close blocks. The whole acquire/use/release path must return within the
// caller's budget, every time.
func TestPeer_RPCDeadlineWithBlockedClose(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAll()

	var (
		connsMu sync.Mutex
		conns   []*blockingCloseConn
		remotes []net.Conn
	)
	defer func() {
		connsMu.Lock()
		defer connsMu.Unlock()
		for _, r := range remotes {
			_ = r.Close()
		}
	}()
	fx.mc.EXPECT().Open(gomock.Any()).DoAndReturn(func(context.Context) (net.Conn, error) {
		in, out := net.Pipe()
		conn := newBlockingCloseConn(in, release)
		connsMu.Lock()
		conns = append(conns, conn)
		remotes = append(remotes, out)
		connsMu.Unlock()
		// the remote handshakes, then swallows the request and never replies
		go func() {
			if _, err := handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker); err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, out)
		}()
		return conn, nil
	}).AnyTimes()

	const (
		budget      = 100 * time.Millisecond
		repetitions = 10
	)
	for i := 0; i < repetitions; i++ {
		cctx, cancel := context.WithTimeout(ctx, budget)
		start := time.Now()
		err := fx.DoDrpc(cctx, func(c drpc.Conn) error {
			stream, err := c.NewStream(cctx, "/test.Test/Call", nil)
			if err != nil {
				return err
			}
			if err = stream.(streamRawWrite).RawWrite(drpcwire.KindMessage, []byte("req")); err != nil {
				return err
			}
			var reply rawMsg
			return stream.MsgRecv(&reply, rawEncoding{})
		})
		elapsed := time.Since(start)
		cancel()
		require.Error(t, err)
		require.Less(t, elapsed, budget+500*time.Millisecond, "repetition %d: the call must return within its budget", i)
		// the cancelled sub conn is never handed out again
		fx.mu.Lock()
		assert.Empty(t, fx.inactive, "repetition %d", i)
		assert.Empty(t, fx.active, "repetition %d", i)
		fx.mu.Unlock()
		// at most one blocked close per repetition
		running, _ := fx.cleanup.stats()
		require.LessOrEqual(t, running, i+1)
	}
	connsMu.Lock()
	require.Len(t, conns, repetitions, "each repetition opens a fresh sub conn")
	connsMu.Unlock()

	// once the transport lets go, every stream is closed and nothing leaks
	releaseAll()
	connsMu.Lock()
	for _, conn := range conns {
		select {
		case <-conn.closed:
		case <-time.After(2 * time.Second):
			t.Fatal("stream was not closed")
		}
	}
	connsMu.Unlock()
	require.Eventually(t, func() bool {
		running, pending := fx.cleanup.stats()
		return running == 0 && pending == 0
	}, 5*time.Second, 10*time.Millisecond, "cleanup workers must exit once idle")
}

// closedConn is a released sub conn that is already closed
type closedConn struct {
	closedCh chan struct{}
	closes   atomic.Int32
}

func newClosedConn() *closedConn {
	ch := make(chan struct{})
	close(ch)
	return &closedConn{closedCh: ch}
}

func (c *closedConn) Close() error               { c.closes.Add(1); return nil }
func (c *closedConn) Closed() <-chan struct{}    { return c.closedCh }
func (c *closedConn) Unblocked() <-chan struct{} { return c.closedCh }
func (c *closedConn) NewStream(context.Context, string, drpc.Encoding) (drpc.Stream, error) {
	return nil, io.EOF
}
func (c *closedConn) Invoke(context.Context, string, drpc.Encoding, drpc.Message, drpc.Message) error {
	return io.EOF
}

func TestPeer_ReleaseClosedAndCancelled(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	// select picks randomly between ready cases: repeat to hit both
	for i := 0; i < 50; i++ {
		conn := newClosedConn()
		sc := &subConn{ConnUnblocked: conn}
		fx.mu.Lock()
		fx.active[sc] = struct{}{}
		fx.mu.Unlock()

		fx.ReleaseDrpcConn(cctx, sc)

		fx.mu.Lock()
		assert.Empty(t, fx.inactive, "a closed or cancelled conn is never reused")
		assert.Empty(t, fx.active)
		fx.mu.Unlock()
		running, pending := fx.cleanup.stats()
		assert.Zero(t, running+pending, "an already closed conn is not queued for cleanup")
		assert.Zero(t, conn.closes.Load())
	}
}

// TestPeer_ReleaseAfterGCDoesNotReuse: a conn gc took out of active is never
// handed out again, even while its close is still pending in the owner
func TestPeer_ReleaseAfterGCDoesNotReuse(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	release := make(chan struct{})
	defer close(release)

	in, out := net.Pipe()
	defer out.Close()
	go func() { _, _ = handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker) }()
	conn := newBlockingCloseConn(in, release)
	fx.mc.EXPECT().Open(gomock.Any()).Return(conn, nil)
	fx.mc.EXPECT().Addr().Return("").AnyTimes()

	dc, err := fx.AcquireDrpcConn(ctx)
	require.NoError(t, err)
	// keep every cleanup worker busy, so the doomed conn's close stays
	// pending and never starts (drpc signals Closed as soon as it does)
	for i := 0; i < cleanupMaxWorkers; i++ {
		fx.cleanup.close(newBlockingCloser(release))
	}
	time.Sleep(20 * time.Millisecond)
	// the idle active conn is doomed; its close waits in the owner
	fx.gc(time.Millisecond)
	select {
	case <-dc.Closed():
		t.Fatal("close was expected to be still pending")
	default:
	}

	fx.ReleaseDrpcConn(ctx, dc)
	fx.mu.Lock()
	assert.Empty(t, fx.inactive, "a doomed conn must not be reused")
	assert.Empty(t, fx.active)
	fx.mu.Unlock()
	select {
	case got := <-fx.subConnRelease:
		require.Nil(t, got, "a doomed conn must not be handed to a waiter")
	default:
	}
}

// racyConn runs gc from inside Unblocked: gc dooms the conn after
// ReleaseDrpcConn has checked the flag, before it takes the peer lock
type racyConn struct {
	p     *peer
	never chan struct{}
	ready chan struct{}
}

func (c *racyConn) Close() error            { return nil }
func (c *racyConn) Closed() <-chan struct{} { return c.never }
func (c *racyConn) Unblocked() <-chan struct{} {
	c.p.gc(time.Millisecond)
	return c.ready
}
func (c *racyConn) NewStream(context.Context, string, drpc.Encoding) (drpc.Stream, error) {
	return nil, io.EOF
}
func (c *racyConn) Invoke(context.Context, string, drpc.Encoding, drpc.Message, drpc.Message) error {
	return io.EOF
}

func TestPeer_ReleaseDoomedDuringCheck(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	fx.mc.EXPECT().Addr().Return("").AnyTimes()
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	rc := &racyConn{p: fx.peer, never: make(chan struct{}), ready: make(chan struct{})}
	close(rc.ready)
	sc := &subConn{ConnUnblocked: rc, LastUsageConn: connutil.NewLastUsageConn(a)}
	fx.mu.Lock()
	fx.active[sc] = struct{}{}
	fx.mu.Unlock()
	time.Sleep(10 * time.Millisecond)

	fx.ReleaseDrpcConn(ctx, sc)
	require.True(t, sc.doomed.Load(), "gc doomed it during the release")
	fx.mu.Lock()
	assert.Empty(t, fx.inactive, "a doomed conn must not be re-pooled")
	fx.mu.Unlock()
}

func TestPeer_AcquireSkipsDoomedConns(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	rc := &racyConn{never: make(chan struct{}), ready: make(chan struct{})}
	doomed := &subConn{ConnUnblocked: rc, LastUsageConn: connutil.NewLastUsageConn(a)}
	doomed.doomed.Store(true)
	fx.mu.Lock()
	fx.inactive = append(fx.inactive, doomed)
	fx.mu.Unlock()

	in, out := net.Pipe()
	defer out.Close()
	go func() { _, _ = handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker) }()
	fx.mc.EXPECT().Open(gomock.Any()).Return(in, nil)
	dc, err := fx.AcquireDrpcConn(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, drpc.Conn(doomed), dc)
}

// TestPeer_NilWakeKeepsThrottling: a waiter woken because a released conn
// was closed must not open at once while closes are still in flight
func TestPeer_NilWakeKeepsThrottling(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	release := make(chan struct{})
	defer close(release)
	// closes stuck in flight on a slow transport, enough for the limiter
	// to hold the next open for seconds
	for i := 0; i < fx.limiter.startThreshold+20; i++ {
		fx.cleanup.close(newBlockingCloser(release))
	}

	var opens atomic.Int32
	fx.mc.EXPECT().Open(gomock.Any()).DoAndReturn(func(context.Context) (net.Conn, error) {
		opens.Add(1)
		return nil, io.EOF
	}).AnyTimes()

	actx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := fx.AcquireDrpcConn(actx)
		done <- err
	}()
	// the waiter is throttled; a release wakes it with a nil conn
	time.Sleep(50 * time.Millisecond)
	fx.subConnRelease <- nil
	require.Never(t, func() bool { return opens.Load() > 0 }, 300*time.Millisecond, 10*time.Millisecond,
		"a nil wake must not bypass the limiter")
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("acquire did not return on ctx")
	}
}

// TestPeer_HungCloseThrottlesAcquireOnlyWhileInFlight: a close that never
// returns counts towards the open limiter while it is in flight, and nothing
// more
func TestPeer_HungCloseThrottlesAcquireOnlyWhileInFlight(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	hang := make(chan struct{})
	var hangOnce sync.Once
	unhang := func() { hangOnce.Do(func() { close(hang) }) }
	defer unhang()
	// one past the limiter threshold: the next open waits slowDownStep
	var hung []*blockingCloser
	for i := 0; i <= fx.limiter.startThreshold; i++ {
		cl := newBlockingCloser(hang)
		hung = append(hung, cl)
		fx.cleanup.close(cl)
	}

	var opens atomic.Int32
	in, out := net.Pipe()
	defer out.Close()
	go func() { _, _ = handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker) }()
	fx.mc.EXPECT().Open(gomock.Any()).DoAndReturn(func(context.Context) (net.Conn, error) {
		opens.Add(1)
		return in, nil
	}).Times(1)

	actx, cancel := context.WithTimeout(ctx, fx.limiter.slowDownStep/2)
	_, err := fx.AcquireDrpcConn(actx)
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded, "throttled while the closes are in flight")
	require.Zero(t, opens.Load())

	unhang()
	for _, cl := range hung {
		<-cl.closed
	}
	require.Eventually(t, func() bool { return fx.cleanup.inFlight() == 0 }, time.Second, time.Millisecond)
	actx, cancel = context.WithTimeout(ctx, fx.limiter.slowDownStep/2)
	defer cancel()
	_, err = fx.AcquireDrpcConn(actx)
	require.NoError(t, err, "no throttling once the closes are done")
	require.Equal(t, int32(1), opens.Load())
}

// pendingConn is a released sub conn that is not closed yet, never unblocks,
// and whose Close blocks until released
type pendingConn struct {
	closedCh   chan struct{}
	release    chan struct{}
	closeCalls atomic.Int32
	once       sync.Once
}

func newPendingConn(release chan struct{}) *pendingConn {
	return &pendingConn{closedCh: make(chan struct{}), release: release}
}

func (c *pendingConn) Close() error {
	c.closeCalls.Add(1)
	<-c.release
	c.once.Do(func() { close(c.closedCh) })
	return nil
}
func (c *pendingConn) Closed() <-chan struct{}    { return c.closedCh }
func (c *pendingConn) Unblocked() <-chan struct{} { return nil }
func (c *pendingConn) NewStream(context.Context, string, drpc.Encoding) (drpc.Stream, error) {
	return nil, io.EOF
}
func (c *pendingConn) Invoke(context.Context, string, drpc.Encoding, drpc.Message, drpc.Message) error {
	return io.EOF
}

// returnsWithin fails the test, instead of hanging it, when fn blocks
func returnsWithin(t *testing.T, d time.Duration, msg string, fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal(msg)
	}
}

func waitClosed(t *testing.T, c *pendingConn) {
	select {
	case <-c.closedCh:
	case <-time.After(time.Second):
		t.Fatal("the conn was never closed")
	}
}

func TestPeer_ReleaseClosesInBackground(t *testing.T) {
	newActive := func(fx *fixture, release chan struct{}) (*subConn, *pendingConn) {
		pc := newPendingConn(release)
		sc := &subConn{ConnUnblocked: pc}
		fx.mu.Lock()
		fx.active[sc] = struct{}{}
		fx.mu.Unlock()
		return sc, pc
	}
	t.Run("cancelled ctx", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		release := make(chan struct{})
		sc, pc := newActive(fx, release)
		cctx, cancel := context.WithCancel(ctx)
		cancel()

		returnsWithin(t, 100*time.Millisecond, "the blocked close must not run on the caller", func() {
			fx.ReleaseDrpcConn(cctx, sc)
		})
		require.Equal(t, 1, fx.cleanup.inFlight())
		fx.mu.Lock()
		assert.Empty(t, fx.inactive)
		assert.Empty(t, fx.active)
		fx.mu.Unlock()

		close(release)
		waitClosed(t, pc)
		require.Eventually(t, func() bool { return fx.cleanup.inFlight() == 0 }, time.Second, time.Millisecond)
	})
	t.Run("never unblocked", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		release := make(chan struct{})
		sc, pc := newActive(fx, release)

		// it waits the 200ms reuse window, then hands the close over
		returnsWithin(t, 300*time.Millisecond, "the blocked close must not run on the caller", func() {
			fx.ReleaseDrpcConn(ctx, sc)
		})
		require.Equal(t, 1, fx.cleanup.inFlight())
		fx.mu.Lock()
		assert.Empty(t, fx.inactive, "an unfinished conn is not reused")
		fx.mu.Unlock()

		close(release)
		waitClosed(t, pc)
	})
}

func TestPeer_GCClosesInBackground(t *testing.T) {
	newSub := func(t *testing.T, release chan struct{}) (*subConn, *pendingConn) {
		a, b := net.Pipe()
		t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
		pc := newPendingConn(release)
		// a LastUsageConn never used reports a zero last usage: expired
		return &subConn{ConnUnblocked: pc, LastUsageConn: connutil.NewLastUsageConn(a)}, pc
	}
	t.Run("expired inactive conn", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		release := make(chan struct{})
		sc, pc := newSub(t, release)
		fx.mu.Lock()
		fx.inactive = append(fx.inactive, sc)
		fx.mu.Unlock()

		returnsWithin(t, 100*time.Millisecond, "gc must not wait on the close", func() {
			fx.gc(time.Millisecond)
		})
		fx.mu.Lock()
		assert.Empty(t, fx.inactive)
		fx.mu.Unlock()
		require.Equal(t, 1, fx.cleanup.inFlight())
		close(release)
		waitClosed(t, pc)
	})
	t.Run("doomed active conn", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		fx.mc.EXPECT().Addr().Return("").AnyTimes()
		release := make(chan struct{})
		sc, pc := newSub(t, release)
		fx.mu.Lock()
		fx.active[sc] = struct{}{}
		fx.mu.Unlock()

		returnsWithin(t, 100*time.Millisecond, "gc must not wait on the close", func() {
			fx.gc(time.Millisecond)
		})
		require.True(t, sc.doomed.Load())
		require.Equal(t, 1, fx.cleanup.inFlight())
		close(release)
		waitClosed(t, pc)
	})
}
