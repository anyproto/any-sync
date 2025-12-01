package periodicsync

import (
	"context"
	"fmt"
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
		err := pSync.Kick(context.Background())
		require.NoError(t, err)
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
		err := pSync.Reset(context.Background())
		require.NoError(t, err)
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

	t.Run("loop with timeout", func(t *testing.T) {
		times := atomic.Int32{}
		calls := make(chan struct{}, 10)
		timeout := 100 * time.Millisecond
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			deadline, ok := ctx.Deadline()
			require.True(t, ok, "context should have deadline when timeout is set")
			require.WithinDuration(t, time.Now().Add(timeout), deadline, 10*time.Millisecond)
			calls <- struct{}{}
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Minute, timeout, diffSyncer, l)
		pSync.Run()
		waitForCall(t, calls)
		require.Equal(t, int32(1), times.Load())
		pSync.Close()
	})

	t.Run("loop caller returns error", func(t *testing.T) {
		times := atomic.Int32{}
		calls := make(chan struct{}, 10)
		testErr := fmt.Errorf("test error")
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			calls <- struct{}{}
			return testErr
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
		// trigger another call via ticker to ensure loop continues after error
		firstTicker.Tick()
		waitForCall(t, calls)
		require.Equal(t, int32(2), times.Load())
		// trigger another call via kick to ensure it still works after error
		err := pSync.Kick(context.Background())
		require.NoError(t, err)
		waitForCall(t, calls)
		require.Equal(t, int32(3), times.Load())
		pSync.Close()
	})

	t.Run("kick context cancelled", func(t *testing.T) {
		times := atomic.Int32{}
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			// block forever to simulate slow processing
			<-ctx.Done()
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l).(*periodicCall)
		pSync.newTicker = func(d time.Duration) ticker {
			return newFakeTicker()
		}
		pSync.Run()
		// wait for initial call to start (it will block)
		time.Sleep(10 * time.Millisecond)
		// try to kick with already cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := pSync.Kick(ctx)
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, int32(1), times.Load())
		pSync.Close()
	})

	t.Run("reset context cancelled", func(t *testing.T) {
		times := atomic.Int32{}
		diffSyncer := func(ctx context.Context) error {
			times.Add(1)
			// block forever to simulate slow processing
			<-ctx.Done()
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l).(*periodicCall)
		pSync.newTicker = func(d time.Duration) ticker {
			return newFakeTicker()
		}
		pSync.Run()
		// wait for initial call to start (it will block)
		time.Sleep(10 * time.Millisecond)
		// try to reset with already cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := pSync.Reset(ctx)
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, int32(1), times.Load())
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
