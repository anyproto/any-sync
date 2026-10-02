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
	"github.com/anyproto/any-sync/net/transport/mock_transport"
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

type writeTimeoutMC struct {
	*mock_transport.MockMultiConn
	wt time.Duration
}

func (w writeTimeoutMC) WriteTimeout() time.Duration { return w.wt }

func TestCleanupOwner(t *testing.T) {
	newMC := func(t *testing.T) (*mock_transport.MockMultiConn, *atomic.Int32) {
		ctrl := gomock.NewController(t)
		mc := mock_transport.NewMockMultiConn(ctrl)
		var mcCloses atomic.Int32
		mc.EXPECT().Close().DoAndReturn(func() error {
			mcCloses.Add(1)
			return nil
		}).AnyTimes()
		return mc, &mcCloses
	}
	t.Run("burst on a healthy connection never closes it", func(t *testing.T) {
		mc, mcCloses := newMC(t)
		c := newCleanupOwner(mc)

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
		assert.Zero(t, c.escalations.Load())
		assert.Zero(t, mcCloses.Load(), "a healthy connection must not be closed")
		assert.LessOrEqual(t, int(maxSeen.Load()), cleanupMaxWorkers)
		// workers exit once idle: an idle peer costs no goroutines
		require.Eventually(t, func() bool {
			r, p := c.stats()
			return r == 0 && p == 0
		}, time.Second, time.Millisecond)
	})
	t.Run("stalled transport closes the multiconn", func(t *testing.T) {
		mc, mcCloses := newMC(t)
		c := newCleanupOwner(mc)
		c.stallTimeout = 50 * time.Millisecond

		release := make(chan struct{})
		var closers []*blockingCloser
		enqueue := func() *blockingCloser {
			cl := newBlockingCloser(release)
			closers = append(closers, cl)
			c.close(cl)
			return cl
		}
		for i := 0; i < cleanupMaxWorkers; i++ {
			enqueue()
		}
		// saturated but not stalled yet: the close just queues
		enqueue()
		r, p := c.stats()
		assert.Equal(t, cleanupMaxWorkers, r)
		assert.Equal(t, 1, p)
		assert.Zero(t, c.escalations.Load())

		time.Sleep(2 * c.stallTimeout)
		start := time.Now()
		enqueue()
		enqueue()
		assert.Less(t, time.Since(start), 100*time.Millisecond, "close must never block")
		assert.Equal(t, int64(2), c.escalations.Load())
		require.Eventually(t, func() bool { return mcCloses.Load() == 1 }, time.Second, time.Millisecond)

		// nothing is dropped, and the multiconn is closed only once
		close(release)
		for _, cl := range closers {
			select {
			case <-cl.closed:
			case <-time.After(time.Second):
				t.Fatal("a queued close was dropped")
			}
		}
		assert.Equal(t, int32(1), mcCloses.Load())
	})
	t.Run("sustained close rate on a healthy connection never closes it", func(t *testing.T) {
		mc, mcCloses := newMC(t)
		c := newCleanupOwner(mc)

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
		assert.Zero(t, c.escalations.Load())
		assert.Zero(t, mcCloses.Load(), "a healthy connection must not be closed")
		// closes in flight are visible to the peer's open limiter
		assert.Greater(t, sawInFlight, cleanupMaxWorkers)
		require.Eventually(t, func() bool { return c.inFlight() == 0 }, time.Second, time.Millisecond)
	})
	t.Run("stall threshold follows the transport write timeout", func(t *testing.T) {
		mc, _ := newMC(t)
		assert.Equal(t, cleanupDefaultStallTimeout, newCleanupOwner(mc).stallTimeout)
		assert.Equal(t, 30*time.Second, newCleanupOwner(writeTimeoutMC{mc, 15 * time.Second}).stallTimeout)
		assert.Equal(t, cleanupDefaultStallTimeout, newCleanupOwner(writeTimeoutMC{mc, 0}).stallTimeout)
	})
	t.Run("nil owner", func(t *testing.T) {
		var c *cleanupOwner
		release := make(chan struct{})
		close(release)
		cl := newBlockingCloser(release)
		c.close(cl)
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
		// at most one blocked close per repetition, nothing escalated
		running, _ := fx.cleanup.stats()
		require.LessOrEqual(t, running, i+1)
	}
	connsMu.Lock()
	require.Len(t, conns, repetitions, "each repetition opens a fresh sub conn")
	connsMu.Unlock()
	require.Zero(t, fx.cleanup.escalations.Load())

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
