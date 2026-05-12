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

func u64ptr(v uint64) *uint64                  { return &v }
func dsPtr(v DeletedStatus) *DeletedStatus     { return &v }

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
