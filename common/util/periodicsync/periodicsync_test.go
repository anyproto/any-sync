package periodicsync

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestPeriodicSync_Run(t *testing.T) {
	// setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	l := logger.NewNamed("sync")

	t.Run("diff syncer 1 time", func(t *testing.T) {
		secs := 0
		times := 0
		diffSyncer := func(ctx context.Context) (err error) {
			times += 1
			return nil
		}
		pSync := NewPeriodicSync(secs, diffSyncer, l)

		pSync.Run()
		pSync.Close()
		require.Equal(t, 1, times)
	})

	t.Run("diff syncer 2 times", func(t *testing.T) {
		secs := 1

		times := 0
		diffSyncer := func(ctx context.Context) (err error) {
			times += 1
			return nil
		}
		pSync := NewPeriodicSync(secs, diffSyncer, l)

		pSync.Run()
		time.Sleep(time.Second * time.Duration(secs))
		pSync.Close()
		require.Equal(t, 2, times)
	})
}
