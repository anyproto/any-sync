package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/quic/mock_quic"
)

func TestClassifyClose(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cause     error
		lifetime  time.Duration
		bytesRead int64
		want      transport.ConnCloseKind
	}{
		{
			name:     "young idle timeout is degraded",
			cause:    &quic.IdleTimeoutError{},
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseDegraded,
		},
		{
			// the censor lets a flow run for a while before freezing it, so
			// bytes having moved is no defence against an idle timeout
			name:      "idle timeout stays degraded however much data moved",
			cause:     &quic.IdleTimeoutError{},
			lifetime:  40 * time.Second,
			bytesRead: 20000,
			want:      transport.ConnCloseDegraded,
		},
		{
			name:     "old idle timeout is healthy",
			cause:    &quic.IdleTimeoutError{},
			lifetime: 6 * time.Minute,
			want:     transport.ConnCloseHealthy,
		},
		{
			name:     "long-lived conn is healthy whatever the cause",
			cause:    &quic.ApplicationError{ErrorCode: 2},
			lifetime: 10 * time.Minute,
			want:     transport.ConnCloseHealthy,
		},
		{
			// the pool closes idle peers after about a minute, so this is the
			// ordinary end of a working connection - and the only evidence of
			// a healthy path that short-lived RPC peers ever produce
			name:      "short conn that carried data and was closed locally is healthy",
			cause:     &quic.ApplicationError{ErrorCode: 2},
			lifetime:  70 * time.Second,
			bytesRead: 4096,
			want:      transport.ConnCloseHealthy,
		},
		{
			name:     "young graceful close with no data is neutral",
			cause:    &quic.ApplicationError{ErrorCode: 2},
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseNeutral,
		},
		{
			// the peer answered us with a reset token, so packets are
			// demonstrably crossing in both directions
			name:     "stateless reset is healthy: the path provably works",
			cause:    &quic.StatelessResetError{},
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseHealthy,
		},
		{
			name:     "young local close is neutral",
			cause:    net.ErrClosed,
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseNeutral,
		},
		{
			name:     "young context cancel is neutral",
			cause:    context.Canceled,
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseNeutral,
		},
		{
			name:     "nil cause is neutral",
			cause:    nil,
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseNeutral,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyClose(tc.cause, tc.lifetime, tc.bytesRead))
		})
	}
}

func TestQuicTransport_DialConnObserver(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	events := make(chan transport.ConnCloseEvent, 1)
	fxC.SetConnObserver(func(ev transport.ConnCloseEvent) { events <- ev })

	mcC, err := fxC.Dial(ctx, fxS.addr)
	require.NoError(t, err)
	require.NoError(t, mcC.Close())

	select {
	case ev := <-events:
		assert.Equal(t, fxS.acc.Account().PeerId, ev.PeerId)
		assert.Equal(t, transport.ConnCloseNeutral, ev.Kind)
	case <-time.After(time.Second * 5):
		t.Fatal("no conn close event")
	}
}

func TestQuicMultiConn_Watch(t *testing.T) {
	newWatchedConn := func(t *testing.T, ctrl *gomock.Controller, startTime time.Time) (*quicMultiConn, context.CancelCauseFunc) {
		connCtx, cancel := context.WithCancelCause(context.Background())
		mockConn := mock_quic.NewMockconnection(ctrl)
		mockConn.EXPECT().Context().Return(connCtx).AnyTimes()
		return &quicMultiConn{
			cctx:       peer.CtxWithPeerId(context.Background(), "p1"),
			connection: mockConn,
			startTime:  startTime,
		}, cancel
	}

	t.Run("degraded death reports peer and counters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		q, cancel := newWatchedConn(t, ctrl, time.Now().Add(-40*time.Second))
		q.bytesRead.Store(100)
		q.bytesWritten.Store(200)

		events := make(chan transport.ConnCloseEvent, 1)
		go q.watch(func(ev transport.ConnCloseEvent) { events <- ev })
		cancel(&quic.IdleTimeoutError{})

		select {
		case ev := <-events:
			assert.Equal(t, transport.ConnCloseDegraded, ev.Kind)
			assert.Equal(t, "p1", ev.PeerId)
			assert.Equal(t, int64(100), ev.BytesRead)
			assert.Equal(t, int64(200), ev.BytesWritten)
			assert.Greater(t, ev.Lifetime, 39*time.Second)
			require.Error(t, ev.Cause)
		case <-time.After(time.Second):
			t.Fatal("no close event")
		}
	})

	t.Run("long-lived death reports healthy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		q, cancel := newWatchedConn(t, ctrl, time.Now().Add(-10*time.Minute))

		events := make(chan transport.ConnCloseEvent, 1)
		go q.watch(func(ev transport.ConnCloseEvent) { events <- ev })
		cancel(&quic.IdleTimeoutError{})

		select {
		case ev := <-events:
			assert.Equal(t, transport.ConnCloseHealthy, ev.Kind)
		case <-time.After(time.Second):
			t.Fatal("no close event")
		}
	})
}

// TestQuicMultiConn_LifetimeSpansSleep pins that a connection's age is
// measured on the wall clock. Go's monotonic clock pauses while the machine
// sleeps, so a monotonic lifetime would report a connection that slept for
// hours as seconds old and classify its wake-up idle timeout as degraded -
// exactly the false positive degradedMaxLifetime exists to exclude.
func TestQuicMultiConn_LifetimeSpansSleep(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mock_quic.NewMockconnection(ctrl)
	mockConn.EXPECT().RemoteAddr().Return(&net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 1112}).AnyTimes()

	mc := newConn(context.Background(), nil, mockConn, time.Second, time.Second).(*quicMultiConn)

	assert.True(t, mc.startTime == mc.startTime.Round(0),
		"startTime must carry no monotonic reading, otherwise sleep is excluded from the lifetime")
}

func TestQuicTransport_SetConnObserverRace(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				fxC.SetConnObserver(func(ev transport.ConnCloseEvent) {})
			}
		}
	}()

	for i := 0; i < 5; i++ {
		mc, err := fxC.Dial(ctx, fxS.addr)
		require.NoError(t, err)
		require.NoError(t, mc.Close())
	}
	close(stop)
	<-done
}

func TestQuicMultiConn_WatchNilObserver(t *testing.T) {
	// the nil check in Dial is not the only thing standing between a server
	// node (which never enables demotion) and a nil call in a detached
	// goroutine
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	connCtx, cancel := context.WithCancelCause(context.Background())
	mockConn := mock_quic.NewMockconnection(ctrl)
	mockConn.EXPECT().Context().Return(connCtx).AnyTimes()
	q := &quicMultiConn{cctx: context.Background(), connection: mockConn, startTime: time.Now().Round(0)}

	done := make(chan struct{})
	go func() {
		defer close(done)
		q.watch(nil)
	}()
	cancel(&quic.IdleTimeoutError{})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watch did not return")
	}
}

// TestQuicGo_BlackholedDialReportsIdleTimeout pins the quic-go behaviour
// IsDialDegraded is built on. quic-go measures its idle deadline from
// connection start while nothing is received, and that deadline is half the
// handshake one - so a UDP path that swallows packets surfaces as an idle
// timeout and never as a handshake timeout. A library bump that changed this
// would silently stop the detector from ever scoring a blocked path.
func TestQuicGo_BlackholedDialReportsIdleTimeout(t *testing.T) {
	// a socket that receives our packets and never answers
	blackhole, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer blackhole.Close()

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	require.NoError(t, err)
	defer udpConn.Close()

	_, err = quic.Dial(context.Background(), udpConn, blackhole.LocalAddr(),
		&tls.Config{InsecureSkipVerify: true, NextProtos: []string{"anysync"}},
		&quic.Config{HandshakeIdleTimeout: 500 * time.Millisecond})
	require.Error(t, err)

	var idle *quic.IdleTimeoutError
	assert.ErrorAs(t, err, &idle, "a blackholed dial must surface as an idle timeout")
	var handshake *quic.HandshakeTimeoutError
	assert.False(t, errors.As(err, &handshake), "and never as a handshake timeout")
	assert.True(t, IsDialDegraded(err))
}
