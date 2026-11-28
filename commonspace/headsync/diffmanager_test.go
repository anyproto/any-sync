package headsync

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
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
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type diffManagerFixture struct {
	ctrl              *gomock.Controller
	storageMock       *mock_spacestorage.MockSpaceStorage
	aclMock           *mock_syncacl.MockSyncAcl
	deletionStateMock *mock_deletionstate.MockObjectDeletionState
	headStorage       *mock_headstorage.MockHeadStorage
	stateStorage      *mock_statestorage.MockStateStorage
	diffMock          *mock_ldiff.MockDiff
	diffManager       *DiffManager
}

func newDiffManagerFixture(t *testing.T) *diffManagerFixture {
	ctrl := gomock.NewController(t)
	storageMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
	aclMock := mock_syncacl.NewMockSyncAcl(ctrl)
	deletionStateMock := mock_deletionstate.NewMockObjectDeletionState(ctrl)
	headStorage := mock_headstorage.NewMockHeadStorage(ctrl)
	stateStorage := mock_statestorage.NewMockStateStorage(ctrl)
	diffMock := mock_ldiff.NewMockDiff(ctrl)

	storageMock.EXPECT().HeadStorage().Return(headStorage).AnyTimes()
	storageMock.EXPECT().StateStorage().Return(stateStorage).AnyTimes()

	log := logger.NewNamed("test")
	diffManager := NewDiffManager(
		diffMock,
		storageMock,
		aclMock,
		log,
		context.Background(),
		deletionStateMock,
	)

	return &diffManagerFixture{
		ctrl:              ctrl,
		storageMock:       storageMock,
		aclMock:           aclMock,
		deletionStateMock: deletionStateMock,
		headStorage:       headStorage,
		stateStorage:      stateStorage,
		diffMock:          diffMock,
		diffManager:       diffManager,
	}
}

func (fx *diffManagerFixture) stop() {
	fx.ctrl.Finish()
}

func (fx *diffManagerFixture) expectIterateEntries(headEntries []headstorage.HeadsEntry) {
	fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, entryIter headstorage.EntryIterator) error {
			for _, entry := range headEntries {
				if res, err := entryIter(entry); err != nil || !res {
					return err
				}
			}
			return nil
		})
}

func TestDiffManager_FillDiff(t *testing.T) {
	ctx := context.Background()

	t.Run("fill diff with entries", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		fx.expectIterateEntries([]headstorage.HeadsEntry{
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
		})

		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		hasher := ldiff.NewHasher()
		hash1 := hasher.HashId("h1h2")
		hash2 := hasher.HashId("h3")
		ldiff.ReleaseHasher(hasher)

		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash1,
		}, ldiff.Element{
			Id:   "id2",
			Head: hash2,
		})

		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.stateStorage.EXPECT().SetHash(ctx, "hash").Return(nil)

		err := fx.diffManager.FillDiff(ctx)
		require.NoError(t, err)
	})

	t.Run("skip derived entries", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		fx.expectIterateEntries([]headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"id1"},
				CommonSnapshot: "snapshot1",
				IsDerived:      true,
			},
		})

		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		// no elements should be set for derived entries
		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.stateStorage.EXPECT().SetHash(ctx, "hash").Return(nil)

		err := fx.diffManager.FillDiff(ctx)
		require.NoError(t, err)
	})

	t.Run("skip singular roots", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		fx.expectIterateEntries([]headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"id1"}, // Singular root
				CommonSnapshot: "snapshot1",
				IsDerived:      false,
			},
			{
				Id:             "id2",
				Heads:          []string{"h1", "h2"},
				CommonSnapshot: "snapshot2",
				IsDerived:      false,
			},
		})

		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		hasher := ldiff.NewHasher()
		hash2 := hasher.HashId("h1h2")
		ldiff.ReleaseHasher(hasher)

		// only id2 should be set
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id2",
			Head: hash2,
		})

		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.stateStorage.EXPECT().SetHash(ctx, "hash").Return(nil)

		err := fx.diffManager.FillDiff(ctx)
		require.NoError(t, err)
	})

	t.Run("include singular root without common snapshot", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		fx.expectIterateEntries([]headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"id1"},
				CommonSnapshot: "",
				IsDerived:      false,
			},
		})

		fx.aclMock.EXPECT().Id().Return("aclId").Times(1)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).Times(1)

		hasher := ldiff.NewHasher()
		hash1 := hasher.HashId("id1")
		ldiff.ReleaseHasher(hasher)

		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash1,
		})

		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.stateStorage.EXPECT().SetHash(ctx, "hash").Return(nil)

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

		fx.diffMock.EXPECT().Hash().Return("hash")

		expectedErr := fmt.Errorf("hash error")
		fx.stateStorage.EXPECT().SetHash(ctx, "hash").Return(expectedErr)

		err := fx.diffManager.FillDiff(ctx)
		require.ErrorIs(t, err, expectedErr)
	})
}

type fakeRemoteDiff struct {
	needsSync bool
	err       error
}

func (f *fakeRemoteDiff) Ranges(ctx context.Context, ranges []ldiff.Range, resBuf []ldiff.RangeResult) ([]ldiff.RangeResult, error) {
	return nil, nil
}

func (f *fakeRemoteDiff) DiffTypeCheck(ctx context.Context, diff ldiff.Diff) (bool, error) {
	return f.needsSync, f.err
}

func TestDiffManager_TryDiff(t *testing.T) {
	ctx := context.Background()

	t.Run("diff with remote", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		remoteDiff := &fakeRemoteDiff{needsSync: true}
		expectedNewIds := []string{"new1", "new2"}
		expectedChangedIds := []string{"changed1"}
		expectedRemovedIds := []string{"removed1"}

		fx.diffMock.EXPECT().DiffBelowRoot(ctx, remoteDiff).Return(expectedNewIds, expectedChangedIds, expectedRemovedIds, nil)

		newIds, changedIds, removedIds, err := fx.diffManager.TryDiff(ctx, remoteDiff)
		require.NoError(t, err)
		require.Equal(t, expectedNewIds, newIds)
		require.Equal(t, expectedChangedIds, changedIds)
		require.Equal(t, expectedRemovedIds, removedIds)
	})

	t.Run("no sync needed", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		remoteDiff := &fakeRemoteDiff{needsSync: false}

		newIds, changedIds, removedIds, err := fx.diffManager.TryDiff(ctx, remoteDiff)
		require.NoError(t, err)
		require.Nil(t, newIds)
		require.Nil(t, changedIds)
		require.Nil(t, removedIds)
	})

	t.Run("diff type check fails", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		expectedErr := fmt.Errorf("check error")
		remoteDiff := &fakeRemoteDiff{err: expectedErr}

		_, _, _, err := fx.diffManager.TryDiff(ctx, remoteDiff)
		require.ErrorIs(t, err, expectedErr)
	})
}

func TestDiffManager_UpdateHeads(t *testing.T) {
	t.Run("delete head", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		deleteStatus := headstorage.DeletedStatusDeleted
		update := headstorage.HeadsEntry{
			Id:            "id1",
			DeletedStatus: deleteStatus,
		}

		fx.diffMock.EXPECT().RemoveId("id1").Return(nil)
		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash").Return(nil)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("update head for regular object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsEntry{
			Id:    "id1",
			Heads: []string{"head1", "head2"},
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)

		hasher := ldiff.NewHasher()
		hash := hasher.HashId("head1head2")
		ldiff.ReleaseHasher(hasher)

		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash").Return(nil)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("skip deleted object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsEntry{
			Id:    "id1",
			Heads: []string{"head1"},
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(true)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("skip derived object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsEntry{
			Id:        "id1",
			Heads:     []string{"id1"},
			IsDerived: true,
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("skip singular root object", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsEntry{
			Id:    "id1",
			Heads: []string{"id1"}, // Singular root
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)

		// no diff update and no hash update for singular roots
		fx.diffManager.UpdateHeads(update)
	})

	t.Run("update head with empty heads array", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsEntry{
			Id:    "id1",
			Heads: []string{}, // Empty heads
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)

		hasher := ldiff.NewHasher()
		hash := hasher.HashId("")
		ldiff.ReleaseHasher(hasher)

		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash").Return(nil)

		fx.diffManager.UpdateHeads(update)
	})

	t.Run("set hash fails but continues", func(t *testing.T) {
		fx := newDiffManagerFixture(t)
		defer fx.stop()

		update := headstorage.HeadsEntry{
			Id:    "id1",
			Heads: []string{"head1"},
		}

		fx.deletionStateMock.EXPECT().Exists("id1").Return(false)

		hasher := ldiff.NewHasher()
		hash := hasher.HashId("head1")
		ldiff.ReleaseHasher(hasher)

		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: hash,
		})
		fx.diffMock.EXPECT().Hash().Return("hash")
		// UpdateHeads logs warning but doesn't fail
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash").Return(fmt.Errorf("hash error"))

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

		_, err := fx.diffManager.HandleRangeRequest(ctx, req)
		require.Error(t, err)
		require.Equal(t, spacesyncproto.ErrUnexpected, err)
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
		fx.diffMock.EXPECT().Ids().Return(expectedIds)

		ids := fx.diffManager.AllIds()
		require.Equal(t, expectedIds, ids)
	})
}

// TestDiffManager_HashStability pins the V3 space hash for a scenario covering
// every head-entry class. The golden value was produced by the pre-V2-removal
// DiffManager (dual old/new diffs) on the same input: if this test fails, the
// change breaks hash compatibility with deployed peers and will force a
// network-wide resync — see the skip conditions in FillDiff and UpdateHeads.
func TestDiffManager_HashStability(t *testing.T) {
	fx := newDiffManagerFixture(t)
	defer fx.stop()

	fx.stateStorage.EXPECT().SetHash(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fx.aclMock.EXPECT().Id().Return("aclId").AnyTimes()
	fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "headId"}).AnyTimes()
	fx.deletionStateMock.EXPECT().Exists(gomock.Any()).Return(false).AnyTimes()

	fx.expectIterateEntries([]headstorage.HeadsEntry{
		{Id: "A", Heads: []string{"h1", "h2"}, CommonSnapshot: "s"},
		{Id: "B", Heads: []string{"h3"}, CommonSnapshot: ""},
		{Id: "C", Heads: []string{"C"}, CommonSnapshot: "s"},
		{Id: "D", Heads: []string{"D"}, CommonSnapshot: "s", IsDerived: true},
		{Id: "E", Heads: []string{"E"}, CommonSnapshot: ""},
	})

	diff := ldiff.New(32, 256)
	dm := NewDiffManager(diff, fx.storageMock, fx.aclMock, logger.NewNamed("test"), context.Background(), fx.deletionStateMock)
	require.NoError(t, dm.FillDiff(context.Background()))
	for _, u := range []headstorage.HeadsEntry{
		{Id: "A", Heads: []string{"h1", "h2", "h5"}},
		{Id: "G", Heads: []string{"G"}},
		{Id: "store", Heads: []string{"k1"}},
		{Id: "B", DeletedStatus: headstorage.DeletedStatusDeleted},
	} {
		dm.UpdateHeads(u)
	}
	require.Equal(t, "9d2101bbe6775d7cd4f1d322d0f227e48ccc27ae3bc745dbda5d05bd4ba61fc1", diff.Hash())
}

// cachedHeadClient answers HeadSync like a node: a hash-only root request from a head cache
// with a placeholder count, everything else from the space diff
type cachedHeadClient struct {
	diff                ldiff.Diff
	calls               int
	rootElementRequests int
	elements            int
}

func (c *cachedHeadClient) HeadSync(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	c.calls++
	if len(req.Ranges) == 1 && !req.Ranges[0].Elements && req.Ranges[0].From == 0 && req.Ranges[0].To == math.MaxUint64 {
		hash, err := hex.DecodeString(c.diff.Hash())
		if err != nil {
			return nil, err
		}
		return &spacesyncproto.HeadSyncResponse{
			DiffType: spacesyncproto.DiffType_V3,
			Results:  []*spacesyncproto.HeadSyncResult{{Hash: hash, Count: 1}},
		}, nil
	}
	for _, rg := range req.Ranges {
		if rg.From == 0 && rg.To == math.MaxUint64 && rg.Elements {
			c.rootElementRequests++
		}
	}
	resp, err := HandleRangeRequest(ctx, c.diff, req)
	if err != nil {
		return nil, err
	}
	for _, res := range resp.Results {
		c.elements += len(res.Elements)
	}
	return resp, nil
}

func TestDiffManager_TryDiff_cachedRoot(t *testing.T) {
	ctx := context.Background()
	elements := func(from, to int) []ldiff.Element {
		els := make([]ldiff.Element, 0, to-from)
		for i := from; i < to; i++ {
			els = append(els, ldiff.Element{Id: fmt.Sprintf("id%d", i), Head: "head"})
		}
		return els
	}
	tryDiff := func(t *testing.T, local, remote []ldiff.Element) (newIds, changedIds, removedIds []string, client *cachedHeadClient) {
		diff := ldiff.New(32, 256)
		diff.Set(local...)
		client = &cachedHeadClient{diff: ldiff.New(32, 256)}
		client.diff.Set(remote...)
		dm := NewDiffManager(diff, nil, nil, logger.NewNamed("test"), ctx, nil)
		newIds, changedIds, removedIds, err := dm.TryDiff(ctx, NewRemoteDiff("spaceId", client))
		require.NoError(t, err)
		return
	}
	t.Run("a large space with a few differences walks child ranges", func(t *testing.T) {
		remote := append(elements(2, 5007), ldiff.Element{Id: "id1", Head: "changed"})
		newIds, changedIds, removedIds, client := tryDiff(t, elements(0, 5000), remote)
		require.Len(t, newIds, 7)
		require.Equal(t, []string{"id1"}, changedIds)
		require.Equal(t, []string{"id0"}, removedIds)
		require.Zero(t, client.rootElementRequests)
		// only the leaves holding a difference are fetched
		require.Less(t, client.elements, 9*256)
	})
	t.Run("an empty space fetches every head in one round", func(t *testing.T) {
		newIds, _, _, client := tryDiff(t, nil, elements(0, 3))
		require.Len(t, newIds, 3)
		require.Equal(t, 3, client.calls)
		require.Equal(t, 3, client.elements)
	})
	t.Run("equal hashes need the check only", func(t *testing.T) {
		newIds, changedIds, removedIds, client := tryDiff(t, elements(0, 1000), elements(0, 1000))
		require.Empty(t, newIds)
		require.Empty(t, changedIds)
		require.Empty(t, removedIds)
		require.Equal(t, 1, client.calls)
	})
	t.Run("a tiny remote", func(t *testing.T) {
		newIds, changedIds, removedIds, _ := tryDiff(t, elements(0, 5000), elements(0, 1))
		require.Empty(t, newIds)
		require.Empty(t, changedIds)
		require.Len(t, removedIds, 4999)
	})
	t.Run("an empty remote", func(t *testing.T) {
		newIds, changedIds, removedIds, _ := tryDiff(t, elements(0, 5000), nil)
		require.Empty(t, newIds)
		require.Empty(t, changedIds)
		require.Len(t, removedIds, 5000)
	})
}
