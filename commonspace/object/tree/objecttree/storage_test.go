package objecttree

import (
	"context"
	"path/filepath"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
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

	t.Run("parent does not exist - returns error", func(t *testing.T) {
		ctx := context.Background()
		store := newTestStore(t)
		hs, err := headstorage.New(ctx, store)
		require.NoError(t, err)

		creator := NewMockChangeCreator(nil)

		// Create child with ParentId pointing to non-existent parent
		childRoot := creator.CreateDerivedRootWithParent("child3", "nonexistent-parent")
		_, err = CreateStorage(ctx, childRoot, hs, store)
		require.ErrorIs(t, err, ErrParentNotFound)
	})

	t.Run("parent is derived - returns error", func(t *testing.T) {
		ctx := context.Background()
		store := newTestStore(t)
		hs, err := headstorage.New(ctx, store)
		require.NoError(t, err)

		creator := NewMockChangeCreator(nil)

		// Create derived parent
		parentRoot := creator.CreateDerivedRoot("derived-parent", true)
		_, err = CreateStorage(ctx, parentRoot, hs, store)
		require.NoError(t, err)

		// Try to create child with derived parent - should fail
		childRoot := creator.CreateDerivedRootWithParent("child4", "derived-parent")
		_, err = CreateStorage(ctx, childRoot, hs, store)
		require.ErrorIs(t, err, ErrDerivedParent)
	})
}
