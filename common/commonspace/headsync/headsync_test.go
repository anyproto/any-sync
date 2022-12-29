package headsync

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/ldiff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/ldiff/mock_ldiff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/headsync/mock_headsync"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage/mock_treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings/deletionstate/mock_deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage/mock_spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/periodicsync/mock_periodicsync"
	"github.com/golang/mock/gomock"
	"testing"
)

func TestDiffService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	spaceId := "spaceId"
	l := logger.NewNamed("sync")
	pSyncMock := mock_periodicsync.NewMockPeriodicSync(ctrl)
	storageMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
	treeStorageMock := mock_treestorage.NewMockTreeStorage(ctrl)
	diffMock := mock_ldiff.NewMockDiff(ctrl)
	syncer := mock_headsync.NewMockDiffSyncer(ctrl)
	delState := mock_deletionstate.NewMockDeletionState(ctrl)
	syncPeriod := 1
	initId := "initId"

	service := &headSync{
		spaceId:      spaceId,
		storage:      storageMock,
		periodicSync: pSyncMock,
		syncer:       syncer,
		diff:         diffMock,
		log:          l,
		syncPeriod:   syncPeriod,
	}

	t.Run("init", func(t *testing.T) {
		storageMock.EXPECT().TreeStorage(initId).Return(treeStorageMock, nil)
		treeStorageMock.EXPECT().Heads().Return([]string{"h1", "h2"}, nil)
		syncer.EXPECT().Init(delState)
		diffMock.EXPECT().Set(ldiff.Element{
			Id:   initId,
			Head: "h1h2",
		})
		pSyncMock.EXPECT().Run()
		service.Init([]string{initId}, delState)
	})

	t.Run("update heads", func(t *testing.T) {
		syncer.EXPECT().UpdateHeads(initId, []string{"h1", "h2"})
		service.UpdateHeads(initId, []string{"h1", "h2"})
	})

	t.Run("remove objects", func(t *testing.T) {
		syncer.EXPECT().RemoveObjects([]string{"h1", "h2"})
		service.RemoveObjects([]string{"h1", "h2"})
	})

	t.Run("close", func(t *testing.T) {
		pSyncMock.EXPECT().Close()
		service.Close()
	})
}
