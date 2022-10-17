package diffservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice/mock_diffservice"
	"github.com/golang/mock/gomock"
	"testing"
	"time"
)

func TestPeriodicSync_Run(t *testing.T) {
	// setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	l := logger.NewNamed("sync")
	diffSyncer := mock_diffservice.NewMockDiffSyncer(ctrl)
	t.Run("diff syncer 1 time", func(t *testing.T) {
		secs := 0
		pSync := newPeriodicSync(secs, diffSyncer, l)

		diffSyncer.EXPECT().Sync(gomock.Any()).Times(1).Return(nil)

		pSync.Run()
		pSync.Close()
	})

	t.Run("diff syncer 2 times", func(t *testing.T) {
		secs := 1

		pSync := newPeriodicSync(secs, diffSyncer, l)
		diffSyncer.EXPECT().Sync(gomock.Any()).Times(2).Return(nil)

		pSync.Run()
		time.Sleep(time.Second * time.Duration(secs))
		pSync.Close()
	})
}
