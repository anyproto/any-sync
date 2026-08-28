package quic

import (
	"context"
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
		name     string
		cause    error
		lifetime time.Duration
		want     transport.ConnCloseKind
	}{
		{
			name:     "young idle timeout is degraded",
			cause:    &quic.IdleTimeoutError{},
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseDegraded,
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
			name:     "young graceful close is neutral",
			cause:    &quic.ApplicationError{ErrorCode: 2},
			lifetime: 40 * time.Second,
			want:     transport.ConnCloseNeutral,
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
			assert.Equal(t, tc.want, classifyClose(tc.cause, tc.lifetime))
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
