package headstorage

import (
	"context"
	"path/filepath"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"
)

func newTestStorage(t *testing.T) (HeadStorage, func()) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := anystore.Open(ctx, dbPath, nil)
	require.NoError(t, err)
	hs, err := New(ctx, db)
	require.NoError(t, err)
	return hs, func() { _ = db.Close() }
}

func seed(t *testing.T, hs HeadStorage, entries []HeadsUpdate) {
	t.Helper()
	for _, u := range entries {
		require.NoError(t, hs.UpdateEntry(context.Background(), u))
	}
}

func collect(t *testing.T, hs HeadStorage, opts IterOpts) []HeadsEntry {
	t.Helper()
	var out []HeadsEntry
	err := hs.IterateEntries(context.Background(), opts, func(e HeadsEntry) (bool, error) {
		out = append(out, e)
		return true, nil
	})
	require.NoError(t, err)
	return out
}

func u64ptr(v uint64) *uint64              { return &v }
func dsPtr(v DeletedStatus) *DeletedStatus { return &v }

func TestIterateEntries_MinLastAddSeq(t *testing.T) {
	hs, cleanup := newTestStorage(t)
	defer cleanup()

	seed(t, hs, []HeadsUpdate{
		{Id: "a", LastAddSeq: u64ptr(10)},
		{Id: "b", LastAddSeq: u64ptr(5)},
		{Id: "c", LastAddSeq: u64ptr(20)},
		{Id: "d", LastAddSeq: u64ptr(15)},
		{Id: "e", LastAddSeq: u64ptr(7)},
	})

	got := collect(t, hs, IterOpts{MinLastAddSeq: 7})
	ids := make([]string, 0, len(got))
	seqs := make([]uint64, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.Id)
		seqs = append(seqs, e.LastAddSeq)
	}
	require.Equal(t, []string{"a", "d", "c"}, ids)
	require.Equal(t, []uint64{10, 15, 20}, seqs)
}

func TestIterateEntries_MinLastAddSeqZero(t *testing.T) {
	hs, cleanup := newTestStorage(t)
	defer cleanup()

	seed(t, hs, []HeadsUpdate{
		{Id: "a", LastAddSeq: u64ptr(10)},
		{Id: "b", LastAddSeq: u64ptr(5)},
		{Id: "c", LastAddSeq: u64ptr(20)},
	})

	got := collect(t, hs, IterOpts{MinLastAddSeq: 0})
	ids := make([]string, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.Id)
	}
	require.Equal(t, []string{"a", "b", "c"}, ids)
}

func TestIterateEntries_MinLastAddSeqWithDeleted(t *testing.T) {
	hs, cleanup := newTestStorage(t)
	defer cleanup()

	seed(t, hs, []HeadsUpdate{
		{Id: "a", LastAddSeq: u64ptr(10)},
		{Id: "b", LastAddSeq: u64ptr(15), DeletedStatus: dsPtr(DeletedStatusQueued)},
		{Id: "c", LastAddSeq: u64ptr(20), DeletedStatus: dsPtr(DeletedStatusDeleted)},
		{Id: "d", LastAddSeq: u64ptr(3), DeletedStatus: dsPtr(DeletedStatusDeleted)},
		{Id: "e", LastAddSeq: u64ptr(25)},
	})

	got := collect(t, hs, IterOpts{Deleted: true, MinLastAddSeq: 10})
	ids := make([]string, 0, len(got))
	seqs := make([]uint64, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.Id)
		seqs = append(seqs, e.LastAddSeq)
	}
	require.Equal(t, []string{"b", "c"}, ids)
	require.Equal(t, []uint64{15, 20}, seqs)

	got = collect(t, hs, IterOpts{Deleted: false, MinLastAddSeq: 5})
	ids = ids[:0]
	for _, e := range got {
		ids = append(ids, e.Id)
	}
	require.Equal(t, []string{"a", "e"}, ids)
}

func TestIterateEntriesWithParent(t *testing.T) {
	hs, cleanup := newTestStorage(t)
	defer cleanup()

	strPtr := func(v string) *string { return &v }
	seed(t, hs, []HeadsUpdate{
		{Id: "a"},
		{Id: "b", ParentId: strPtr("a")},
		{Id: "c", ParentId: strPtr("a"), DeletedStatus: dsPtr(DeletedStatusDeleted)},
		{Id: "d", DeletedStatus: dsPtr(DeletedStatusDeleted)},
		{Id: "e", ParentId: strPtr("d"), DeletedStatus: dsPtr(DeletedStatusQueued)},
	})

	var got []HeadsEntry
	err := hs.IterateEntriesWithParent(context.Background(), func(e HeadsEntry) (bool, error) {
		got = append(got, e)
		return true, nil
	})
	require.NoError(t, err)
	byId := map[string]HeadsEntry{}
	for _, e := range got {
		byId[e.Id] = e
	}
	require.Len(t, byId, 3)
	require.Equal(t, "a", byId["b"].ParentId)
	require.Equal(t, DeletedStatusNotDeleted, byId["b"].DeletedStatus)
	require.Equal(t, "a", byId["c"].ParentId)
	require.Equal(t, DeletedStatusDeleted, byId["c"].DeletedStatus)
	require.Equal(t, "d", byId["e"].ParentId)
	require.Equal(t, DeletedStatusQueued, byId["e"].DeletedStatus)

	// stops when the iterator returns false
	var count int
	err = hs.IterateEntriesWithParent(context.Background(), func(e HeadsEntry) (bool, error) {
		count++
		return false, nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// answered from the sparse parent id index
	explain, err := hs.(*headStorage).headsColl.Find(hasParentFilter).Explain(context.Background())
	require.NoError(t, err)
	var used bool
	for _, idx := range explain.Indexes {
		if idx.Name == parentIdKey && idx.Used {
			used = true
		}
	}
	require.True(t, used, explain.Indexes)
}
