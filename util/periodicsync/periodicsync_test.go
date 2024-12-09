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
		diffSyncer := func(ctx context.Context) (err error) {
			times.Add(1)
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Second, 0, diffSyncer, l)
		pSync.Run()
		pSync.Close()
		require.Equal(t, int32(1), times.Load())
	})

	t.Run("loop call 2 times", func(t *testing.T) {
		var neededTimes int32 = 2
		times := atomic.Int32{}
		ch := make(chan struct{})
		diffSyncer := func(ctx context.Context) (err error) {
			times.Add(1)
			if neededTimes == times.Load() {
				close(ch)
			}
			return nil
		}
		pSync := NewPeriodicSyncDuration(time.Millisecond*100, 0, diffSyncer, l)
		pSync.Run()
		<-ch
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
