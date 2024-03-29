package periodicsync

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
	"time"
)

func TestPeriodicSync_Run(t *testing.T) {
	// setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	l := logger.NewNamed("sync")

	t.Run("loop call 1 time", func(t *testing.T) {
		secs := 0
		times := 0
		diffSyncer := func(ctx context.Context) (err error) {
			times += 1
			return nil
		}
		pSync := NewPeriodicSync(secs, 0, diffSyncer, l)

		pSync.Run()
		pSync.Close()
		require.Equal(t, 1, times)
	})

	t.Run("loop call 2 times", func(t *testing.T) {
		secs := 1

		times := 0
		diffSyncer := func(ctx context.Context) (err error) {
			times += 1
			return nil
		}
		pSync := NewPeriodicSync(secs, 0, diffSyncer, l)

		pSync.Run()
		time.Sleep(time.Second * time.Duration(secs))
		pSync.Close()
		require.Equal(t, 2, times)
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
