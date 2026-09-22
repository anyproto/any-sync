package objecttree

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage/mock_headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func newTestStore(t *testing.T) anystore.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := anystore.Open(context.Background(), dbPath, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestCreateStorageLateArrivingChild(t *testing.T) {
	StorageChangeBuilder = func(keys crypto.KeyStorage, rootChange *treechangeproto.RawTreeChangeWithId) ChangeBuilder {
		return &nonVerifiableChangeBuilder{
			ChangeBuilder: NewChangeBuilder(newMockKeyStorage(), rootChange),
		}
	}

	t.Run("parent already queued for deletion - child gets queued", func(t *testing.T) {
		ctx := context.Background()
		store := newTestStore(t)
		hs, err := headstorage.New(ctx, store)
		require.NoError(t, err)

		creator := NewMockChangeCreator(nil)

		// Create non-derived parent tree first
		parentRoot := creator.CreateRoot("parent1", "aclHead")
		_, err = CreateStorage(ctx, parentRoot, hs, store)
		require.NoError(t, err)

		// Mark parent as queued for deletion
		deletedStatus := headstorage.DeletedStatusQueued
		err = hs.UpdateEntry(ctx, headstorage.HeadsUpdate{
			Id:            "parent1",
			DeletedStatus: &deletedStatus,
		})
		require.NoError(t, err)

		// Create child with ParentId pointing to the deleted parent
		childRoot := creator.CreateDerivedRootWithParent("child1", "parent1")
		_, err = CreateStorage(ctx, childRoot, hs, store)
		require.NoError(t, err)

		// Verify child is queued for deletion
		childEntry, err := hs.GetEntry(ctx, "child1")
		require.NoError(t, err)
		require.Equal(t, headstorage.DeletedStatusQueued, childEntry.DeletedStatus)
	})

	t.Run("parent not deleted - child is not queued", func(t *testing.T) {
		ctx := context.Background()
		store := newTestStore(t)
		hs, err := headstorage.New(ctx, store)
		require.NoError(t, err)

		creator := NewMockChangeCreator(nil)

		// Create non-derived parent tree (not deleted)
		parentRoot := creator.CreateRoot("parent2", "aclHead")
		_, err = CreateStorage(ctx, parentRoot, hs, store)
		require.NoError(t, err)

		// Create child with ParentId pointing to active parent
		childRoot := creator.CreateDerivedRootWithParent("child2", "parent2")
		_, err = CreateStorage(ctx, childRoot, hs, store)
		require.NoError(t, err)

		// Verify child is NOT queued for deletion
		childEntry, err := hs.GetEntry(ctx, "child2")
		require.NoError(t, err)
		require.Equal(t, headstorage.DeletedStatusNotDeleted, childEntry.DeletedStatus)
	})

	t.Run("parent does not exist - child stored with the binding", func(t *testing.T) {
		ctx := context.Background()
		store := newTestStore(t)
		hs, err := headstorage.New(ctx, store)
		require.NoError(t, err)

		creator := NewMockChangeCreator(nil)

		// The parent may arrive later or from another peer; the child
		// must not wait for it.
		childRoot := creator.CreateDerivedRootWithParent("child3", "later-parent")
		_, err = CreateStorage(ctx, childRoot, hs, store)
		require.NoError(t, err)

		childEntry, err := hs.GetEntry(ctx, "child3")
		require.NoError(t, err)
		require.Equal(t, "later-parent", childEntry.ParentId)
		require.Equal(t, headstorage.DeletedStatusNotDeleted, childEntry.DeletedStatus)

		// The binding is queryable before the parent exists, so a later
		// parent deletion still cascades.
		children, err := hs.GetEntriesByParentId(ctx, "later-parent")
		require.NoError(t, err)
		require.Len(t, children, 1)
		require.Equal(t, "child3", children[0].Id)
	})

	t.Run("parent is derived - stored; the rule is enforced at creation", func(t *testing.T) {
		ctx := context.Background()
		store := newTestStore(t)
		hs, err := headstorage.New(ctx, store)
		require.NoError(t, err)

		creator := NewMockChangeCreator(nil)

		parentRoot := creator.CreateDerivedRoot("derived-parent", true)
		_, err = CreateStorage(ctx, parentRoot, hs, store)
		require.NoError(t, err)

		// Replication stores what it is given, in any arrival order;
		// objecttreebuilder.DeriveTree refuses this at creation.
		childRoot := creator.CreateDerivedRootWithParent("child4", "derived-parent")
		_, err = CreateStorage(ctx, childRoot, hs, store)
		require.NoError(t, err)
	})

	t.Run("parent lookup error - propagates", func(t *testing.T) {
		ctx := context.Background()
		store := newTestStore(t)
		hs := mock_headstorage.NewMockHeadStorage(gomock.NewController(t))
		boom := errors.New("boom")
		hs.EXPECT().GetEntry(gomock.Any(), "parent5").Return(headstorage.HeadsEntry{}, boom)

		creator := NewMockChangeCreator(nil)

		// Only "not found" means the parent has not arrived; anything
		// else must not be mistaken for it.
		childRoot := creator.CreateDerivedRootWithParent("child5", "parent5")
		_, err := CreateStorage(ctx, childRoot, hs, store)
		require.ErrorIs(t, err, boom)
	})
}
