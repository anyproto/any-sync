package periodicsync

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app/logger"
)

func TestPeriodicSync_Run(t *testing.T) {
	l := logger.NewNamed("sync")

	t.Run("loop call 1 time", func(t *testing.T) {
		synctest.Run(func() {
			times := atomic.Int32{}
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				return nil
			}
			pSync := NewPeriodicSyncDuration(time.Second, 0, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			pSync.Close()
			require.Equal(t, int32(1), times.Load())
		})
	})

	t.Run("loop kick", func(t *testing.T) {
		synctest.Run(func() {
			times := atomic.Int32{}
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				return nil
			}
			pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			require.Equal(t, int32(1), times.Load())
			err := pSync.Kick(context.Background())
			require.NoError(t, err)
			synctest.Wait()
			require.Equal(t, int32(2), times.Load())
			pSync.Close()
		})
	})

	t.Run("loop reset", func(t *testing.T) {
		synctest.Run(func() {
			times := atomic.Int32{}
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				return nil
			}
			pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			require.Equal(t, int32(1), times.Load())
			// advance time to trigger ticker
			time.Sleep(time.Minute)
			synctest.Wait()
			require.Equal(t, int32(2), times.Load())
			// reset should call and restart ticker
			err := pSync.Reset(context.Background())
			require.NoError(t, err)
			synctest.Wait()
			require.Equal(t, int32(3), times.Load())
			// after reset, ticker should fire again after another minute
			time.Sleep(time.Minute)
			synctest.Wait()
			require.Equal(t, int32(4), times.Load())
			pSync.Close()
		})
	})

	t.Run("loop call 2 times", func(t *testing.T) {
		synctest.Run(func() {
			times := atomic.Int32{}
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				return nil
			}
			pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			require.Equal(t, int32(1), times.Load())
			// advance time to trigger ticker
			time.Sleep(time.Minute)
			synctest.Wait()
			require.Equal(t, int32(2), times.Load())
			pSync.Close()
		})
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
		synctest.Run(func() {
			times := atomic.Int32{}
			timeout := 100 * time.Millisecond
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				deadline, ok := ctx.Deadline()
				require.True(t, ok, "context should have deadline when timeout is set")
				require.WithinDuration(t, time.Now().Add(timeout), deadline, 10*time.Millisecond)
				return nil
			}
			pSync := NewPeriodicSyncDuration(time.Minute, timeout, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			require.Equal(t, int32(1), times.Load())
			pSync.Close()
		})
	})

	t.Run("loop caller returns error", func(t *testing.T) {
		synctest.Run(func() {
			times := atomic.Int32{}
			testErr := fmt.Errorf("test error")
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				return testErr
			}
			pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			require.Equal(t, int32(1), times.Load())
			// trigger another call via ticker to ensure loop continues after error
			time.Sleep(time.Minute)
			synctest.Wait()
			require.Equal(t, int32(2), times.Load())
			// trigger another call via kick to ensure it still works after error
			err := pSync.Kick(context.Background())
			require.NoError(t, err)
			synctest.Wait()
			require.Equal(t, int32(3), times.Load())
			pSync.Close()
		})
	})

	t.Run("kick context cancelled", func(t *testing.T) {
		synctest.Run(func() {
			times := atomic.Int32{}
			blocker := make(chan struct{})
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				// block to simulate slow processing
				<-blocker
				return nil
			}
			pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			// initial call is blocked, try to kick with cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := pSync.Kick(ctx)
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, int32(1), times.Load())
			// unblock and close
			close(blocker)
			synctest.Wait()
			pSync.Close()
		})
	})

	t.Run("reset context cancelled", func(t *testing.T) {
		synctest.Run(func() {
			times := atomic.Int32{}
			blocker := make(chan struct{})
			diffSyncer := func(ctx context.Context) error {
				times.Add(1)
				// block to simulate slow processing
				<-blocker
				return nil
			}
			pSync := NewPeriodicSyncDuration(time.Minute, 0, diffSyncer, l)
			pSync.Run()
			synctest.Wait()
			// initial call is blocked, try to reset with cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := pSync.Reset(ctx)
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, int32(1), times.Load())
			// unblock and close
			close(blocker)
			synctest.Wait()
			pSync.Close()
		})
	})
}
