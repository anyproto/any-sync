package peer

import (
	"context"
	"errors"
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
	"storj.io/drpc/drpcerr"
	"storj.io/drpc/drpcwire"

	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/net/transport"
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
	d      time.Duration
	closed chan struct{}
}

func (q *quickCloser) Close() error {
	time.Sleep(q.d)
	close(q.closed)
	return nil
}

// sendWake hands v to a waiter blocked in AcquireDrpcConn, failing the test
// instead of hanging it when nobody is waiting
func sendWake(t *testing.T, fx *fixture, v drpc.Conn) {
	select {
	case fx.subConnRelease <- v:
	case <-time.After(5 * time.Second):
		t.Fatal("no waiter took the wake-up")
	}
}

// waitWaiter waits until an AcquireDrpcConn call sits in the throttle select
func waitWaiter(t *testing.T, fx *fixture) {
	require.Eventually(t, func() bool { return fx.openingWaitCount.Load() == 1 }, 5*time.Second, time.Millisecond,
		"the caller is not throttled")
}

func TestPeer_CloseAsync(t *testing.T) {
	t.Run("never blocks the caller, never drops a close", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		hang := make(chan struct{})
		defer close(hang)
		hung := newBlockingCloser(hang)
		returnsWithin(t, time.Second, "close must never block", func() { fx.closeAsync(hung, true) })

		// a hung close holds up no other close
		var closers []*quickCloser
		returnsWithin(t, time.Second, "close must never block", func() {
			for i := 0; i < 300; i++ {
				cl := &quickCloser{d: time.Millisecond, closed: make(chan struct{})}
				closers = append(closers, cl)
				fx.closeAsync(cl, true)
			}
		})
		for _, cl := range closers {
			select {
			case <-cl.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("a close was dropped or held up")
			}
		}
		require.Eventually(t, func() bool { return fx.churnClosing.Load() == 1 }, time.Second, time.Millisecond,
			"only the hung close is still counted")
	})
	t.Run("only release closes are counted", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		release := make(chan struct{})
		counted, uncounted := newBlockingCloser(release), newBlockingCloser(release)
		fx.closeAsync(counted, true)
		fx.closeAsync(uncounted, false)
		assert.Equal(t, int32(1), fx.churnClosing.Load())
		close(release)
		<-counted.closed
		<-uncounted.closed
		require.Eventually(t, func() bool { return fx.churnClosing.Load() == 0 }, time.Second, time.Millisecond)
	})
	t.Run("slow close is logged once", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		var logged atomic.Int32
		fx.slowClose = 20 * time.Millisecond
		fx.onSlowClose = func() { logged.Add(1) }
		release := make(chan struct{})
		cl := newBlockingCloser(release)
		fx.closeAsync(cl, false)
		require.Eventually(t, func() bool { return logged.Load() == 1 }, time.Second, time.Millisecond)
		require.Never(t, func() bool { return logged.Load() > 1 }, 100*time.Millisecond, 10*time.Millisecond)
		close(release)
		<-cl.closed
		// a quick close is not logged
		quick := newBlockingCloser(release)
		fx.closeAsync(quick, false)
		<-quick.closed
		require.Never(t, func() bool { return logged.Load() > 1 }, 50*time.Millisecond, 10*time.Millisecond)
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

		returnsWithin(t, 3*time.Second, "the blocked close must not run on the caller", func() {
			_, err := fx.AcquireDrpcConn(ctx)
			assert.ErrorIs(t, err, handshake.ErrRemoteIncompatibleProto)
		})
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
		returnsWithin(t, 3*time.Second, "the blocked close must not run on the caller", func() {
			_, err := fx.AcquireDrpcConn(actx)
			assert.ErrorIs(t, err, context.DeadlineExceeded)
		})

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
		require.Less(t, elapsed, budget+2*time.Second, "repetition %d: the call must return within its budget", i)
		// the cancelled sub conn is never handed out again
		fx.mu.Lock()
		assert.Empty(t, fx.inactive, "repetition %d", i)
		assert.Empty(t, fx.active, "repetition %d", i)
		fx.mu.Unlock()
		// at most one blocked close per repetition
		require.LessOrEqual(t, int(fx.churnClosing.Load()), i+1)
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
	require.Eventually(t, func() bool { return fx.churnClosing.Load() == 0 },
		5*time.Second, 10*time.Millisecond, "background closes must finish")
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
		assert.Zero(t, fx.churnClosing.Load(), "an already closed conn is not closed again")
		assert.Zero(t, conn.closes.Load())
	}
}

// TestPeer_ReleaseAfterGCDoesNotReuse: a conn gc took out of active is never
// handed out again, even while its close is still pending
func TestPeer_ReleaseAfterGCDoesNotReuse(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	fx.mc.EXPECT().Addr().Return("").AnyTimes()
	release := make(chan struct{})
	defer close(release)
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	// a conn that would be reusable (unblocked), whose close blocks and does
	// not report Closed until it returns
	pc := newPendingConn(release)
	pc.unblocked = make(chan struct{})
	close(pc.unblocked)
	sc := &subConn{ConnUnblocked: pc, LastUsageConn: connutil.NewLastUsageConn(a)}
	fx.mu.Lock()
	fx.active[sc] = struct{}{}
	fx.mu.Unlock()

	// the idle active conn is doomed; its close is pending
	fx.gc(time.Millisecond)
	require.True(t, sc.doomed.Load())

	fx.ReleaseDrpcConn(ctx, sc)
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

	fx.ReleaseDrpcConn(ctx, sc)
	require.True(t, sc.doomed.Load(), "gc doomed it during the release")
	fx.mu.Lock()
	assert.Empty(t, fx.inactive, "a doomed conn must not be re-pooled")
	fx.mu.Unlock()
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
		fx.closeAsync(newBlockingCloser(release), true)
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
	waitWaiter(t, fx)
	sendWake(t, fx, nil)
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
	// fifty past the limiter threshold: the next open waits 50 slowDownSteps
	var hung []*blockingCloser
	for i := 0; i < fx.limiter.startThreshold+50; i++ {
		cl := newBlockingCloser(hang)
		hung = append(hung, cl)
		fx.closeAsync(cl, true)
	}

	var opens atomic.Int32
	in, out := net.Pipe()
	defer out.Close()
	go func() { _, _ = handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker) }()
	fx.mc.EXPECT().Open(gomock.Any()).DoAndReturn(func(context.Context) (net.Conn, error) {
		opens.Add(1)
		return in, nil
	}).Times(1)

	actx, cancel := context.WithTimeout(ctx, 10*fx.limiter.slowDownStep)
	_, err := fx.AcquireDrpcConn(actx)
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded, "throttled while the closes are in flight")
	require.Zero(t, opens.Load())

	unhang()
	for _, cl := range hung {
		<-cl.closed
	}
	require.Eventually(t, func() bool { return fx.churnClosing.Load() == 0 }, time.Second, time.Millisecond)
	actx, cancel = context.WithTimeout(ctx, 20*fx.limiter.slowDownStep)
	defer cancel()
	_, err = fx.AcquireDrpcConn(actx)
	require.NoError(t, err, "no throttling once the closes are done")
	require.Equal(t, int32(1), opens.Load())
}

// pendingConn is a released sub conn that is not closed yet, never unblocks,
// and whose Close blocks until released
type pendingConn struct {
	closedCh   chan struct{}
	unblocked  chan struct{}
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
func (c *pendingConn) Unblocked() <-chan struct{} { return c.unblocked }
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

		returnsWithin(t, time.Second, "the blocked close must not run on the caller", func() {
			fx.ReleaseDrpcConn(cctx, sc)
		})
		require.Equal(t, int32(1), fx.churnClosing.Load())
		fx.mu.Lock()
		assert.Empty(t, fx.inactive)
		assert.Empty(t, fx.active)
		fx.mu.Unlock()

		close(release)
		waitClosed(t, pc)
		require.Eventually(t, func() bool { return fx.churnClosing.Load() == 0 }, time.Second, time.Millisecond)
	})
	t.Run("never unblocked", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		release := make(chan struct{})
		sc, pc := newActive(fx, release)

		// it waits the 200ms reuse window, then hands the close over
		returnsWithin(t, time.Second, "the blocked close must not run on the caller", func() {
			fx.ReleaseDrpcConn(ctx, sc)
		})
		require.Equal(t, int32(1), fx.churnClosing.Load())
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

		returnsWithin(t, time.Second, "gc must not wait on the close", func() {
			fx.gc(time.Millisecond)
		})
		fx.mu.Lock()
		assert.Empty(t, fx.inactive)
		fx.mu.Unlock()
		require.Zero(t, fx.churnClosing.Load(), "gc closes are not counted by the limiter")
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

		returnsWithin(t, time.Second, "gc must not wait on the close", func() {
			fx.gc(time.Millisecond)
		})
		require.True(t, sc.doomed.Load())
		require.Zero(t, fx.churnClosing.Load(), "gc closes are not counted by the limiter")
		close(release)
		waitClosed(t, pc)
	})
}

// openPipe makes Open return a fresh handshaking conn each time
func openPipe(t *testing.T, fx *fixture) {
	fx.mc.EXPECT().Open(gomock.Any()).DoAndReturn(func(context.Context) (net.Conn, error) {
		in, out := net.Pipe()
		t.Cleanup(func() { _ = out.Close() })
		go func() { _, _ = handshake.IncomingProtoHandshake(ctx, out, defaultProtoChecker) }()
		return in, nil
	}).AnyTimes()
}

// TestPeer_WakeUpsDoNotStarveWaiters: waiters woken again and again by
// releases keep their first throttling deadline instead of restarting it
func TestPeer_WakeUpsDoNotStarveWaiters(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	openPipe(t, fx)

	// released conns closing slowly, about 10 in flight: a 1s throttle
	stop := make(chan struct{})
	var churn sync.WaitGroup
	churn.Add(1)
	go func() {
		defer churn.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			release := make(chan struct{})
			fx.closeAsync(newBlockingCloser(release), true)
			time.AfterFunc(2*time.Second, func() { close(release) })
			// the release wakes a waiter with a nil conn
			select {
			case fx.subConnRelease <- nil:
			default:
			}
		}
	}()
	defer func() {
		close(stop)
		churn.Wait()
	}()
	for i := 0; i < fx.limiter.startThreshold+10; i++ {
		release := make(chan struct{})
		fx.closeAsync(newBlockingCloser(release), true)
		time.AfterFunc(2*time.Second, func() { close(release) })
	}

	const waiters = 5
	results := make(chan error, waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			actx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			_, err := fx.AcquireDrpcConn(actx)
			results <- err
		}()
	}
	for i := 0; i < waiters; i++ {
		select {
		case err := <-results:
			require.NoError(t, err, "a waiter starved")
		case <-time.After(30 * time.Second):
			t.Fatal("waiter did not return")
		}
	}
}

// TestPeer_GCDoesNotThrottleOpens: closes started by gc are not counted by
// the open limiter, so a gc pass does not delay opens on a healthy peer
func TestPeer_GCDoesNotThrottleOpens(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	openPipe(t, fx)
	release := make(chan struct{})
	defer close(release)
	// expired inactive conns whose closes hang
	for i := 0; i < fx.limiter.startThreshold+50; i++ {
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()
		fx.mu.Lock()
		fx.inactive = append(fx.inactive, &subConn{ConnUnblocked: newPendingConn(release), LastUsageConn: connutil.NewLastUsageConn(a)})
		fx.mu.Unlock()
	}
	fx.gc(time.Millisecond)
	fx.mu.Lock()
	require.Empty(t, fx.inactive)
	fx.mu.Unlock()

	// counted, these closes would hold the open for 50 slowDownSteps
	actx, cancel := context.WithTimeout(ctx, 20*fx.limiter.slowDownStep)
	defer cancel()
	_, err := fx.AcquireDrpcConn(actx)
	require.NoError(t, err, "gc closes must not throttle opens")
}

// TestPeer_FailedHandshakeChurnIsThrottled: closes of sub conns that failed
// the handshake count towards the open limiter, so a peer whose handshakes
// keep failing on a stalled transport cannot spin up opens (and hung close
// goroutines) at the callers' rate
func TestPeer_FailedHandshakeChurnIsThrottled(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	release := make(chan struct{})
	defer close(release)
	var opens atomic.Int32
	fx.mc.EXPECT().Open(gomock.Any()).DoAndReturn(func(context.Context) (net.Conn, error) {
		opens.Add(1)
		in, out := net.Pipe()
		go func() {
			// the remote declines the protocol, then goes away
			_, _ = handshake.IncomingProtoHandshake(ctx, out, handshake.ProtoChecker{AllowedProtoTypes: []handshakeproto.ProtoType{100}})
			_ = out.Close()
		}()
		return newBlockingCloseConn(in, release), nil
	}).AnyTimes()

	// callers retry as fast as they can for a second
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		actx, cancel := context.WithDeadline(ctx, deadline)
		_, _ = fx.AcquireDrpcConn(actx)
		cancel()
	}
	// unthrottled this is hundreds; throttled, past the 10-conn threshold
	// each open waits 100ms more than the one before
	assert.Less(t, int(opens.Load()), 30)
	assert.Equal(t, opens.Load(), fx.churnClosing.Load(), "every failed open's close is counted while it hangs")
}

// TestPeer_WakeWithDoomedConnIsNotHandedOut: gc can doom a released conn
// after ReleaseDrpcConn checked it and before the hand-off to a waiter; the
// waiter must not take it
func TestPeer_WakeWithDoomedConnIsNotHandedOut(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	release := make(chan struct{})
	defer close(release)
	for i := 0; i < fx.limiter.startThreshold+20; i++ {
		fx.closeAsync(newBlockingCloser(release), true)
	}
	fx.mc.EXPECT().Open(gomock.Any()).Return(nil, io.EOF).AnyTimes()

	actx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		conn drpc.Conn
		err  error
	}
	done := make(chan result, 1)
	go func() {
		conn, err := fx.AcquireDrpcConn(actx)
		done <- result{conn, err}
	}()
	waitWaiter(t, fx)
	doomed := &subConn{ConnUnblocked: newPendingConn(release)}
	doomed.doomed.Store(true)
	sendWake(t, fx, doomed)
	// the waiter starts over and keeps waiting rather than using it
	waitWaiter(t, fx)
	cancel()
	select {
	case res := <-done:
		require.ErrorIs(t, res.err, context.Canceled)
		require.Nil(t, res.conn)
	case <-time.After(5 * time.Second):
		t.Fatal("acquire did not return on ctx")
	}
}

// TestPeer_ForeignConnReleaseCloseIsCounted: a released conn that is not a
// sub conn of this peer is closed in the background and counted
func TestPeer_ForeignConnReleaseCloseIsCounted(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	release := make(chan struct{})
	foreign := &foreignConn{pendingConn: newPendingConn(release)}
	returnsWithin(t, time.Second, "the blocked close must not run on the caller", func() {
		fx.ReleaseDrpcConn(ctx, foreign)
	})
	require.Equal(t, int32(1), fx.churnClosing.Load())
	close(release)
	waitClosed(t, foreign.pendingConn)
	require.Eventually(t, func() bool { return fx.churnClosing.Load() == 0 }, time.Second, time.Millisecond)
}

// foreignConn is a drpc.Conn without Unblocked
type foreignConn struct {
	pendingConn *pendingConn
}

func (c *foreignConn) Close() error            { return c.pendingConn.Close() }
func (c *foreignConn) Closed() <-chan struct{} { return c.pendingConn.Closed() }
func (c *foreignConn) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (drpc.Stream, error) {
	return c.pendingConn.NewStream(ctx, rpc, enc)
}
func (c *foreignConn) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) error {
	return c.pendingConn.Invoke(ctx, rpc, enc, in, out)
}

// TestPeer_HandedOffConnIsNotTakenByGC: a conn handed straight to a waiter
// counts as just used, so a gc pass right after does not doom it under the
// new holder, however long it sat idle before
func TestPeer_HandedOffConnIsNotTakenByGC(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	fx.mc.EXPECT().Addr().Return("").AnyTimes()
	release := make(chan struct{})
	defer close(release)
	for i := 0; i < fx.limiter.startThreshold+20; i++ {
		fx.closeAsync(newBlockingCloser(release), true)
	}
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	// never read or written: idle since forever
	sc := &subConn{ConnUnblocked: newPendingConn(release), LastUsageConn: connutil.NewLastUsageConn(a)}
	fx.mu.Lock()
	fx.active[sc] = struct{}{}
	fx.mu.Unlock()

	got := make(chan drpc.Conn, 1)
	go func() {
		dc, err := fx.AcquireDrpcConn(ctx)
		assert.NoError(t, err)
		got <- dc
	}()
	waitWaiter(t, fx)
	sendWake(t, fx, sc)
	select {
	case dc := <-got:
		require.Equal(t, drpc.Conn(sc), dc)
	case <-time.After(5 * time.Second):
		t.Fatal("the waiter did not take the conn")
	}
	fx.gc(time.Minute)
	assert.False(t, sc.doomed.Load(), "gc must not take a conn its holder has just acquired")
	fx.mu.Lock()
	_, active := fx.active[sc]
	fx.mu.Unlock()
	assert.True(t, active)
}

// scriptedConn is a sub conn whose Invoke returns a set error and whose
// Closed fires only when told
type scriptedConn struct {
	closedCh  chan struct{}
	invokeErr error
}

func (c *scriptedConn) Close() error               { return nil }
func (c *scriptedConn) Closed() <-chan struct{}    { return c.closedCh }
func (c *scriptedConn) Unblocked() <-chan struct{} { return nil }
func (c *scriptedConn) NewStream(context.Context, string, drpc.Encoding) (drpc.Stream, error) {
	return nil, c.invokeErr
}
func (c *scriptedConn) Invoke(context.Context, string, drpc.Encoding, drpc.Message, drpc.Message) error {
	return c.invokeErr
}

func TestSubConn_ConnLost(t *testing.T) {
	errClosed := drpc.ClosedError.New("closed")
	t.Run("doomed before its close lands", func(t *testing.T) {
		sc := &subConn{ConnUnblocked: &scriptedConn{closedCh: make(chan struct{}), invokeErr: errClosed}}
		sc.doomed.Store(true)
		err := sc.Invoke(ctx, "/x", nil, nil, nil)
		assert.ErrorIs(t, err, transport.ErrConnClosed)
		assert.ErrorIs(t, err, errClosed)
	})
	t.Run("live sub conn passes errors through", func(t *testing.T) {
		sc := &subConn{ConnUnblocked: &scriptedConn{closedCh: make(chan struct{}), invokeErr: errClosed}}
		assert.Equal(t, errClosed, sc.Invoke(ctx, "/x", nil, nil, nil))
	})
	t.Run("other errors are untouched on a closed sub conn", func(t *testing.T) {
		closed := make(chan struct{})
		close(closed)
		appErr := errors.New("application error")
		for _, cause := range []error{io.EOF, appErr} {
			sc := &subConn{ConnUnblocked: &scriptedConn{closedCh: closed, invokeErr: cause}}
			assert.True(t, sc.Invoke(ctx, "/x", nil, nil, nil) == cause, "%v must be returned as is", cause)
		}
	})
	t.Run("a stream's normal end stays exactly io.EOF", func(t *testing.T) {
		closed := make(chan struct{})
		close(closed)
		sc := &subConn{ConnUnblocked: &scriptedConn{closedCh: closed}}
		st := connLostStream{Stream: eofStream{}, sc: sc, ctx: ctx}
		assert.True(t, st.MsgRecv(nil, nil) == io.EOF, "callers compare with ==")
		assert.True(t, st.MsgSend(nil, nil) == io.EOF)
		assert.True(t, st.CloseSend() == io.EOF)
	})
	t.Run("a coded server reply is never a connection loss", func(t *testing.T) {
		closed := make(chan struct{})
		close(closed)
		coded := drpcerr.WithCode(errors.New("space is deleted"), 1003)
		sc := &subConn{ConnUnblocked: &scriptedConn{closedCh: closed, invokeErr: coded}}
		err := sc.Invoke(ctx, "/x", nil, nil, nil)
		assert.Equal(t, coded, err)
		assert.Equal(t, uint64(1003), drpcerr.Code(err))
	})
	t.Run("the caller's own cancellation stays", func(t *testing.T) {
		closed := make(chan struct{})
		close(closed)
		sc := &subConn{ConnUnblocked: &scriptedConn{closedCh: closed, invokeErr: context.Canceled}}
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		assert.Equal(t, context.Canceled, sc.Invoke(cctx, "/x", nil, nil, nil))
	})
}

// eofStream is a drpc stream the server has ended normally
type eofStream struct{ drpc.Stream }

func (eofStream) MsgRecv(drpc.Message, drpc.Encoding) error { return io.EOF }
func (eofStream) MsgSend(drpc.Message, drpc.Encoding) error { return io.EOF }
func (eofStream) CloseSend() error                          { return io.EOF }
