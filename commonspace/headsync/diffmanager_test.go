package headsync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/ldiff/mock_ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/deletionstate/mock_deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage/mock_headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/statestorage/mock_statestorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/mock_syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/mock_keyvaluestorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces/mock_kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type diffManagerFixture struct {
	ctrl              *gomock.Controller
	diffContainerMock *mock_ldiff.MockDiffContainer
	storageMock       *mock_spacestorage.MockSpaceStorage
	aclMock           *mock_syncacl.MockSyncAcl
	deletionStateMock *mock_deletionstate.MockObjectDeletionState
	kvMock            *mock_kvinterfaces.MockKeyValueService
	defStoreMock      *mock_keyvaluestorage.MockStorage
	headStorage       *mock_headstorage.MockHeadStorage
	stateStorage      *mock_statestorage.MockStateStorage
	diffMock          *mock_ldiff.MockDiff
	diffManager       *DiffManager
}

func newDiffManagerFixture(t *testing.T) *diffManagerFixture {
	ctrl := gomock.NewController(t)
	diffContainerMock := mock_ldiff.NewMockDiffContainer(ctrl)
	storageMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
	aclMock := mock_syncacl.NewMockSyncAcl(ctrl)
	deletionStateMock := mock_deletionstate.NewMockObjectDeletionState(ctrl)
	kvMock := mock_kvinterfaces.NewMockKeyValueService(ctrl)
	defStoreMock := mock_keyvaluestorage.NewMockStorage(ctrl)
	headStorage := mock_headstorage.NewMockHeadStorage(ctrl)
	stateStorage := mock_statestorage.NewMockStateStorage(ctrl)
	diffMock := mock_ldiff.NewMockDiff(ctrl)

	kvMock.EXPECT().DefaultStore().Return(defStoreMock).AnyTimes()
	defStoreMock.EXPECT().Id().Return("store").AnyTimes()
	storageMock.EXPECT().HeadStorage().Return(headStorage).AnyTimes()
	storageMock.EXPECT().StateStorage().Return(stateStorage).AnyTimes()

	log := logger.NewNamed("test")
	diffManager := NewDiffManager(
		diffContainerMock,
		storageMock,
		aclMock,
		log,
		context.Background(),
		deletionStateMock,
		kvMock,
	)

	return &diffManagerFixture{
		ctrl:              ctrl,
		diffContainerMock: diffContainerMock,
		storageMock:       storageMock,
		aclMock:           aclMock,
		deletionStateMock: deletionStateMock,
		kvMock:            kvMock,
		defStoreMock:      defStoreMock,
		headStorage:       headStorage,
		stateStorage:      stateStorage,
		diffMock:          diffMock,
		diffManager:       diffManager,
	}
}

func (fx *diffManagerFixture) stop() {
	fx.ctrl.Finish()
}

func TestDiffManager_FillDiff(t *testing.T) {
	ctx := context.Background()

	t.Run("fill diff with entries", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		headEntries := []headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"h1", "h2"},
				CommonSnapshot: "snapshot1",
				IsDerived:      false,
			},
			{
				Id:             "id2",
				Heads:          []string{"h3"},
				CommonSnapshot: "",
				IsDerived:      false,
			},
		}

		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, entryIter headstorage.EntryIterator) error {
				for _, entry := range headEntries {
					if res, err := entryIter(entry); err != nil || !res {
						return err
					}
				}
				return nil
			})

		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		// Test expects hashed values now
		hasher := ldiff.NewHasher()
		hash1 := hasher.HashId("h1h2")
		hash2 := hasher.HashId("h3")
		ldiff.ReleaseHasher(hasher)

		// NewDiff gets both common and no common snapshot elements
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock).Times(2)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash1,
		})
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id2",
			Head: hash2,
		})

		// OldDiff gets common elements with hash and no common snapshot elements without hash
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock).Times(2)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash1,
		})
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id2",
			Head: "h3",
		})

		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("oldHash")
		fx.diffMock.EXPECT().Hash().Return("newHash")
		fx.stateStorage.EXPECT().SetHash(ctx, "oldHash", "newHash").Return(nil)

		err := fx.diffManager.FillDiff(ctx)
		require.NoError(t, err)
	})

	t.Run("skip derived entries", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		headEntries := []headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"id1"},
				CommonSnapshot: "snapshot1",
				IsDerived:      true,
			},
		}

		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, entryIter headstorage.EntryIterator) error {
				for _, entry := range headEntries {
					if res, err := entryIter(entry); err != nil || !res {
						return err
					}
				}
				return nil
			})

		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		// No elements should be set for derived entries
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("oldHash")
		fx.diffMock.EXPECT().Hash().Return("newHash")
		fx.stateStorage.EXPECT().SetHash(ctx, "oldHash", "newHash").Return(nil)

		err := fx.diffManager.FillDiff(ctx)
		require.NoError(t, err)
	})

	t.Run("handle singular roots", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		headEntries := []headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"id1"},  // Singular root
				CommonSnapshot: "snapshot1",
				IsDerived:      false,
			},
			{
				Id:             "id2",
				Heads:          []string{"h1", "h2"},
				CommonSnapshot: "snapshot2",
				IsDerived:      false,
			},
		}

		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, entryIter headstorage.EntryIterator) error {
				for _, entry := range headEntries {
					if res, err := entryIter(entry); err != nil || !res {
						return err
					}
				}
				return nil
			})

		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		hasher := ldiff.NewHasher()
		hash1 := hasher.HashId("id1")
		hash2 := hasher.HashId("h1h2")
		ldiff.ReleaseHasher(hasher)

		// NewDiff should only get id2
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id2",
			Head: hash2,
		})

		// OldDiff should get both id1 and id2
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock).Times(2)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id2",
			Head: hash2,
		})
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash1,
		})

		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("oldHash")
		fx.diffMock.EXPECT().Hash().Return("newHash")
		fx.stateStorage.EXPECT().SetHash(ctx, "oldHash", "newHash").Return(nil)

		err := fx.diffManager.FillDiff(ctx)
		require.NoError(t, err)
	})

	t.Run("error handling - iterate entries fails", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		expectedErr := fmt.Errorf("iterate error")
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr)

		err := fx.diffManager.FillDiff(ctx)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("error handling - set hash fails", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("oldHash")
		fx.diffMock.EXPECT().Hash().Return("newHash")
		
		expectedErr := fmt.Errorf("hash error")
		fx.stateStorage.EXPECT().SetHash(ctx, "oldHash", "newHash").Return(expectedErr)

		err := fx.diffManager.FillDiff(ctx)
		require.ErrorIs(t, err, expectedErr)
	})
}

func TestDiffManager_TryDiff(t *testing.T) {
	ctx := context.Background()

	t.Run("diff with remote", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		remoteDiff := &remote{spaceId: "space1"}
		expectedNewIds := []string{"new1", "new2"}
		expectedChangedIds := []string{"changed1"}
		expectedRemovedIds := []string{"removed1"}

		fx.diffContainerMock.EXPECT().DiffTypeCheck(ctx, remoteDiff).Return(true, fx.diffMock, nil)
		fx.diffMock.EXPECT().Diff(ctx, remoteDiff).Return(expectedNewIds, expectedChangedIds, expectedRemovedIds, nil)

		newIds, changedIds, removedIds, needsSync, err := fx.diffManager.TryDiff(ctx, remoteDiff)
		require.NoError(t, err)
		require.True(t, needsSync)
		require.Equal(t, expectedNewIds, newIds)
		require.Equal(t, expectedChangedIds, changedIds)
		require.Equal(t, expectedRemovedIds, removedIds)
	})

	t.Run("no sync needed", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		remoteDiff := &remote{spaceId: "space1"}

		fx.diffContainerMock.EXPECT().DiffTypeCheck(ctx, remoteDiff).Return(false, nil, nil)

		newIds, changedIds, removedIds, needsSync, err := fx.diffManager.TryDiff(ctx, remoteDiff)
		require.NoError(t, err)
		require.False(t, needsSync)
		require.Nil(t, newIds)
		require.Nil(t, changedIds)
		require.Nil(t, removedIds)
	})
}

func TestDiffManager_UpdateHeads(t *testing.T) {
	t.Run("delete head", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		deleteStatus := headstorage.DeletedStatusDeleted
		update := headstorage.HeadsUpdate{
			Id:            "id1",
			DeletedStatus: &deleteStatus,
		}

		fx.diffContainerMock.EXPECT().RemoveId("id1").Return(nil)
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("hash").Times(2)
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("update head for key-value store", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsUpdate{
			Id:    "store",
			Heads: []string{"head1"},
		}

		fx.deletionStateMock.EXPECT().Exists("store").Return(false)
		
		hasher := ldiff.NewHasher()
		hash := hasher.HashId("head1")
		ldiff.ReleaseHasher(hasher)

		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "store",
			Head: hash,
		})
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "store",
			Head: "head1",
		})
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("hash").Times(2)
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("update head for regular object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsUpdate{
			Id:    "id1",
			Heads: []string{"head1", "head2"},
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)
		
		hasher := ldiff.NewHasher()
		hash := hasher.HashId("head1head2")
		ldiff.ReleaseHasher(hasher)

		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("hash").Times(2)
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("skip deleted object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsUpdate{
			Id:    "id1",
			Heads: []string{"head1"},
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(true)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("skip derived object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		isDerived := true
		update := headstorage.HeadsUpdate{
			Id:        "id1",
			Heads:     []string{"id1"},
			IsDerived: &isDerived,
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("update singular root object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsUpdate{
			Id:    "id1",
			Heads: []string{"id1"},  // Singular root
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)
		
		hasher := ldiff.NewHasher()
		hash := hasher.HashId("id1")
		ldiff.ReleaseHasher(hasher)

		// Singular roots should only be added to OldDiff and then return early
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		// No hash update since the function returns early for singular roots

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("update head with empty heads array", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsUpdate{
			Id:    "id1",
			Heads: []string{},  // Empty heads
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)
		
		hasher := ldiff.NewHasher()
		hash := hasher.HashId("")
		ldiff.ReleaseHasher(hasher)

		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("hash").Times(2)
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("set hash fails but continues", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsUpdate{
			Id:    "id1",
			Heads: []string{"head1"},
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)
		
		hasher := ldiff.NewHasher()
		hash := hasher.HashId("head1")
		ldiff.ReleaseHasher(hasher)

		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().Return("hash").Times(2)
		// UpdateHeads logs warning but doesn't fail
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(fmt.Errorf("hash error"))

		// Should not panic or fail
		fx.diffManager.UpdateHeads(update)
	})
}

func TestDiffManager_HandleRangeRequest(t *testing.T) {
	ctx := context.Background()

	t.Run("handle range request with V3 diff type", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		req := &spacesyncproto.HeadSyncRequest{
			DiffType: spacesyncproto.DiffType_V3,
		}

		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().DiffType().Return(spacesyncproto.DiffType_V3)
		fx.diffMock.EXPECT().Ranges(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		_, err := fx.diffManager.HandleRangeRequest(ctx, req)
		require.NoError(t, err)
	})

	t.Run("handle range request with V2 diff type", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		req := &spacesyncproto.HeadSyncRequest{
			DiffType: spacesyncproto.DiffType_V2,
		}

		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().DiffType().Return(spacesyncproto.DiffType_V2)
		fx.diffMock.EXPECT().Ranges(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		_, err := fx.diffManager.HandleRangeRequest(ctx, req)
		require.NoError(t, err)
	})

	t.Run("handle range request with unsupported diff type", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		req := &spacesyncproto.HeadSyncRequest{
			DiffType: spacesyncproto.DiffType_V1,
		}

		_, err := fx.diffManager.HandleRangeRequest(ctx, req)
		require.Error(t, err)
		require.Equal(t, spacesyncproto.ErrUnexpected, err)
	})
}

func TestDiffManager_AllIds(t *testing.T) {
	t.Run("get all ids", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		expectedIds := []string{"id1", "id2", "id3"}
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Ids().Return(expectedIds)

		ids := fx.diffManager.AllIds()
		require.Equal(t, expectedIds, ids)
	})
}

