package diffservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice/mock_diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage/mock_storage"
	mock_storage2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage/mock_storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff/mock_ldiff"
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
	storageMock := mock_storage.NewMockSpaceStorage(ctrl)
	treeStorageMock := mock_storage2.NewMockTreeStorage(ctrl)
	diffMock := mock_ldiff.NewMockDiff(ctrl)
	syncer := mock_diffservice.NewMockDiffSyncer(ctrl)
	delState := deletionstate.NewDeletionState(storageMock)
	syncPeriod := 1
	initId := "initId"

	service := &diffService{
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
