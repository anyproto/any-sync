package periodicsync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app/logger"
)

func TestPeriodicSync_Run(t *testing.T) {
	// setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	l := logger.NewNamed("sync")

	t.Run("loop call 1 time", func(t *testing.T) {
		times := atomic.Int32{}
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Second, 0, diffSyncer, l)
		pSync.Run()
		pSync.Close()
		require.Equal(t, int32(1), times.Load())
	})
	t.Run("loop kick", func(t *testing.T) {
		times := atomic.Int32{}
		calls := make(chan struct{}, 10)
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			calls <- struct{}{}
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l).(*periodicCall)
		tickerCh := make(chan *fakeTicker, 1)
		pSync.newTicker = func(d time.Duration) ticker {
			ft := newFakeTicker()
			tickerCh <- ft
			return ft
		}
		pSync.Run()
		<-tickerCh // ensure ticker created so we can control ticks if needed
		waitForCall(t, calls)
		require.Equal(t, int32(1), times.Load())
		pSync.Kick()
		waitForCall(t, calls)
		require.Equal(t, int32(2), times.Load())
		pSync.Close()
	})

	t.Run("loop reset", func(t *testing.T) {
		times := atomic.Int32{}
		calls := make(chan struct{}, 10)
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			calls <- struct{}{}
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l).(*periodicCall)
		tickerCh := make(chan *fakeTicker, 2)
		pSync.newTicker = func(d time.Duration) ticker {
			ft := newFakeTicker()
			tickerCh <- ft
			return ft
		}
		pSync.Run()
		firstTicker := <-tickerCh
		waitForCall(t, calls)
		firstTicker.Tick()
		waitForCall(t, calls)
		require.Equal(t, int32(2), times.Load())
		pSync.Reset()
		waitForCall(t, calls)
		require.Equal(t, int32(3), times.Load())
		require.True(t, firstTicker.Stopped())
		secondTicker := <-tickerCh
		require.NotSame(t, firstTicker, secondTicker)
		secondTicker.Tick()
		waitForCall(t, calls)
		require.Equal(t, int32(4), times.Load())
		pSync.Close()
	})
	t.Run("loop call 2 times", func(t *testing.T) {
		var neededTimes int32 = 2
		times := atomic.Int32{}
		calls := make(chan struct{}, 10)
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			if neededTimes >= times.Load() {
				calls <- struct{}{}
			}
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l).(*periodicCall)
		tickerCh := make(chan *fakeTicker, 1)
		pSync.newTicker = func(d time.Duration) ticker {
			ft := newFakeTicker()
			tickerCh <- ft
			return ft
		}
		pSync.Run()
		firstTicker := <-tickerCh
		waitForCall(t, calls)
		require.Equal(t, int32(1), times.Load())
		firstTicker.Tick()
		waitForCall(t, calls)
		require.Equal(t, int32(2), times.Load())
		pSync.Close()
	})

	t.Run("loop close not running", func(t *testing.T) {
		secs := 0
		diffSyncer := func(ctx context.Context) (err error) {
			return nil
		}
		pSync := NewPeriodicSync(secs, 0, diffSyncer, l)
		pSync.Close()
	})
}

func waitForCall(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for periodic call")
	}
}

type fakeTicker struct {
	ch      chan time.Time
	stopped atomic.Bool
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{ch: make(chan time.Time)}
}

func (f *fakeTicker) C() <-chan time.Time {
	return f.ch
}

func (f *fakeTicker) Stop() {
	if f.stopped.Swap(true) {
		return
	}
	close(f.ch)
}

func (f *fakeTicker) Tick() {
	if f.stopped.Load() {
		panic("tick called on stopped ticker")
	}
	f.ch <- time.Now()
}

func (f *fakeTicker) Stopped() bool {
	return f.stopped.Load()
}
