package objecttree

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func init() {
	StorageChangeBuilder = func(keys crypto.KeyStorage, rootChange *treechangeproto.RawTreeChangeWithId) ChangeBuilder {
		return &nonVerifiableChangeBuilder{
			ChangeBuilder: NewChangeBuilder(newMockKeyStorage(), rootChange),
		}
	}
}

func createTestStorage(t *testing.T, treeId string) (Storage, headstorage.HeadStorage) {
	t.Helper()
	ctx := context.Background()
	store := newTestStore(t)
	hs, err := headstorage.New(ctx, store)
	require.NoError(t, err)

	creator := NewMockChangeCreator(nil)
	root := creator.CreateRoot(treeId, "aclHead")
	st, err := CreateStorage(ctx, root, hs, store)
	require.NoError(t, err)
	return st, hs
}

func TestAddSeq_AddAll(t *testing.T) {
	ctx := context.Background()
	st, _ := createTestStorage(t, "tree1")

	// Set the addSeq counter
	var counter atomic.Uint64
	st.(*storage).SetAddSeq(&counter)

	// First AddAll call - add two changes, both should get seq=1
	changes1 := []StorageChange{
		{Id: "ch1", OrderId: "b", RawChange: []byte("data1"), SnapshotId: "tree1", SnapshotCounter: 1},
		{Id: "ch2", OrderId: "c", RawChange: []byte("data2"), SnapshotId: "tree1", SnapshotCounter: 1},
	}
	err := st.AddAll(ctx, changes1, []string{"ch1", "ch2"}, "tree1")
	require.NoError(t, err)

	// Verify both changes got seq=1
	ch1, err := st.Get(ctx, "ch1")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), ch1.AddSeq)

	ch2, err := st.Get(ctx, "ch2")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), ch2.AddSeq)

	// Second AddAll call - should get seq=2
	changes2 := []StorageChange{
		{Id: "ch3", OrderId: "d", RawChange: []byte("data3"), SnapshotId: "tree1", SnapshotCounter: 1},
	}
	err = st.AddAll(ctx, changes2, []string{"ch3"}, "tree1")
	require.NoError(t, err)

	ch3, err := st.Get(ctx, "ch3")
	require.NoError(t, err)
	assert.Equal(t, uint64(2), ch3.AddSeq)

	// Counter should be at 2
	assert.Equal(t, uint64(2), counter.Load())
}

func TestAddSeq_AddAllNoError(t *testing.T) {
	ctx := context.Background()
	st, _ := createTestStorage(t, "tree1")

	var counter atomic.Uint64
	st.(*storage).SetAddSeq(&counter)

	// First AddAll call
	changes1 := []StorageChange{
		{Id: "ch1", OrderId: "b", RawChange: []byte("data1"), SnapshotId: "tree1", SnapshotCounter: 1},
	}
	err := st.AddAllNoError(ctx, changes1, []string{"ch1"}, "tree1")
	require.NoError(t, err)

	ch1, err := st.Get(ctx, "ch1")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), ch1.AddSeq)

	// Insert duplicate - AddAllNoError should not fail on ErrDocExists
	changes2 := []StorageChange{
		{Id: "ch1", OrderId: "b", RawChange: []byte("data1"), SnapshotId: "tree1", SnapshotCounter: 1},
		{Id: "ch2", OrderId: "c", RawChange: []byte("data2"), SnapshotId: "tree1", SnapshotCounter: 1},
	}
	err = st.AddAllNoError(ctx, changes2, []string{"ch1", "ch2"}, "tree1")
	require.NoError(t, err)

	// seq should have been incremented for the second call
	assert.Equal(t, uint64(2), counter.Load())

	ch2, err := st.Get(ctx, "ch2")
	require.NoError(t, err)
	assert.Equal(t, uint64(2), ch2.AddSeq)
}

func TestAddSeq_NoopCounter(t *testing.T) {
	ctx := context.Background()
	st, _ := createTestStorage(t, "tree1")

	// Use a zero-valued counter (like objecttreevalidator does for ephemeral storages)
	var noop atomic.Uint64
	st.(*storage).SetAddSeq(&noop)

	changes := []StorageChange{
		{Id: "ch1", OrderId: "b", RawChange: []byte("data1"), SnapshotId: "tree1", SnapshotCounter: 1},
		{Id: "ch2", OrderId: "c", RawChange: []byte("data2"), SnapshotId: "tree1", SnapshotCounter: 1},
	}
	err := st.AddAll(ctx, changes, []string{"ch1", "ch2"}, "tree1")
	require.NoError(t, err)

	// All changes get seq=1 (single Add(1) per batch)
	ch1, err := st.Get(ctx, "ch1")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), ch1.AddSeq)

	ch2, err := st.Get(ctx, "ch2")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), ch2.AddSeq)
}

func TestAddSeq_GetAfterAddSeq(t *testing.T) {
	ctx := context.Background()
	st, _ := createTestStorage(t, "tree1")

	var counter atomic.Uint64
	st.(*storage).SetAddSeq(&counter)

	// Add changes across three AddAll calls: seq=1, seq=2, seq=3
	err := st.AddAll(ctx, []StorageChange{
		{Id: "ch1", OrderId: "b", RawChange: []byte("d1"), SnapshotId: "tree1", SnapshotCounter: 1},
	}, []string{"ch1"}, "tree1")
	require.NoError(t, err)

	err = st.AddAll(ctx, []StorageChange{
		{Id: "ch2", OrderId: "c", RawChange: []byte("d2"), SnapshotId: "tree1", SnapshotCounter: 1},
		{Id: "ch3", OrderId: "d", RawChange: []byte("d3"), SnapshotId: "tree1", SnapshotCounter: 1},
	}, []string{"ch2", "ch3"}, "tree1")
	require.NoError(t, err)

	err = st.AddAll(ctx, []StorageChange{
		{Id: "ch4", OrderId: "e", RawChange: []byte("d4"), SnapshotId: "tree1", SnapshotCounter: 1},
	}, []string{"ch4"}, "tree1")
	require.NoError(t, err)

	// GetAfterAddSeq(0) should return all changes with seq > 0 (ch1, ch2, ch3, ch4)
	var results []StorageChange
	err = st.GetAfterAddSeq(ctx, 0, func(ctx context.Context, change StorageChange) (bool, error) {
		results = append(results, change)
		return true, nil
	})
	require.NoError(t, err)
	assert.Len(t, results, 4)

	// GetAfterAddSeq(1) should return changes with seq > 1 (ch2, ch3, ch4)
	results = nil
	err = st.GetAfterAddSeq(ctx, 1, func(ctx context.Context, change StorageChange) (bool, error) {
		results = append(results, change)
		return true, nil
	})
	require.NoError(t, err)
	assert.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.AddSeq > 1)
	}

	// GetAfterAddSeq(2) should return changes with seq > 2 (ch4 only)
	results = nil
	err = st.GetAfterAddSeq(ctx, 2, func(ctx context.Context, change StorageChange) (bool, error) {
		results = append(results, change)
		return true, nil
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "ch4", results[0].Id)
	assert.Equal(t, uint64(3), results[0].AddSeq)

	// GetAfterAddSeq(3) should return nothing
	results = nil
	err = st.GetAfterAddSeq(ctx, 3, func(ctx context.Context, change StorageChange) (bool, error) {
		results = append(results, change)
		return true, nil
	})
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestAddSeq_GetAfterAddSeq_Empty(t *testing.T) {
	ctx := context.Background()
	st, _ := createTestStorage(t, "tree1")

	var counter atomic.Uint64
	st.(*storage).SetAddSeq(&counter)

	// No additional changes added (only root with seq=0)
	// Query for seq > 0 should return nothing since root was created without addSeq
	var results []StorageChange
	err := st.GetAfterAddSeq(ctx, 0, func(ctx context.Context, change StorageChange) (bool, error) {
		results = append(results, change)
		return true, nil
	})
	require.NoError(t, err)
	assert.Len(t, results, 0)

	// Query for seq > 100 on an empty store should also return nothing
	results = nil
	err = st.GetAfterAddSeq(ctx, 100, func(ctx context.Context, change StorageChange) (bool, error) {
		results = append(results, change)
		return true, nil
	})
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestAddSeq_Parallel(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	hs, err := headstorage.New(ctx, store)
	require.NoError(t, err)

	var counter atomic.Uint64
	creator := NewMockChangeCreator(nil)

	const numGoroutines = 10

	// Create separate tree storages, each sharing the same counter
	storages := make([]Storage, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		treeId := fmt.Sprintf("tree-%d", i)
		root := creator.CreateRoot(treeId, "aclHead")
		st, err := CreateStorage(ctx, root, hs, store)
		require.NoError(t, err)
		st.(*storage).SetAddSeq(&counter)
		storages[i] = st
	}

	// Each goroutine does one AddAll call
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			treeId := fmt.Sprintf("tree-%d", idx)
			changeId := fmt.Sprintf("change-%d", idx)
			changes := []StorageChange{
				{Id: changeId, OrderId: "b", RawChange: []byte("data"), SnapshotId: treeId, SnapshotCounter: 1},
			}
			err := storages[idx].AddAll(ctx, changes, []string{changeId}, treeId)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// Final counter value should equal number of AddAll calls
	assert.Equal(t, uint64(numGoroutines), counter.Load())

	// Collect all assigned seq values
	seqSet := make(map[uint64]bool)
	for i := 0; i < numGoroutines; i++ {
		changeId := fmt.Sprintf("change-%d", i)
		ch, err := storages[i].Get(ctx, changeId)
		require.NoError(t, err)
		assert.True(t, ch.AddSeq > 0, "expected seq > 0, got %d", ch.AddSeq)
		seqSet[ch.AddSeq] = true
	}

	// No duplicate seq values
	assert.Len(t, seqSet, numGoroutines, "expected all seq values to be unique")
}

func TestAddSeq_DeferredCreation(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	hs, err := headstorage.New(ctx, store)
	require.NoError(t, err)

	creator := NewMockChangeCreator(nil)
	root := creator.CreateRoot("deferred-tree", "aclHead")

	st, err := CreateStorageWithDeferredCreation(ctx, root, hs, store)
	require.NoError(t, err)

	// Set addSeq on the deferred storage
	var counter atomic.Uint64
	st.(*storageDeferredCreation).SetAddSeq(&counter)

	// AddAll should trigger actual creation and propagate addSeq
	changes := []StorageChange{
		{Id: "ch1", OrderId: "b", RawChange: []byte("data1"), SnapshotId: "deferred-tree", SnapshotCounter: 1},
	}
	err = st.AddAll(ctx, changes, []string{"ch1"}, "deferred-tree")
	require.NoError(t, err)

	// Counter should be incremented
	assert.Equal(t, uint64(1), counter.Load())

	// The change should have the correct seq
	ch1, err := st.Get(ctx, "ch1")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), ch1.AddSeq)

	// Verify that the underlying storage now exists and has addSeq set
	// by doing another AddAll (which goes directly to the underlying storage)
	changes2 := []StorageChange{
		{Id: "ch2", OrderId: "c", RawChange: []byte("data2"), SnapshotId: "deferred-tree", SnapshotCounter: 1},
	}
	err = st.AddAll(ctx, changes2, []string{"ch2"}, "deferred-tree")
	require.NoError(t, err)

	assert.Equal(t, uint64(2), counter.Load())

	ch2, err := st.Get(ctx, "ch2")
	require.NoError(t, err)
	assert.Equal(t, uint64(2), ch2.AddSeq)
}
