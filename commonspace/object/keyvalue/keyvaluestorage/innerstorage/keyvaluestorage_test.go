package innerstorage_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
)

var ctx = context.Background()

// failingHeadStorage lets a test fail the head update inside Set's transaction.
type failingHeadStorage struct {
	headstorage.HeadStorage
	fail bool
}

func (f *failingHeadStorage) UpdateEntry(ctx context.Context, update headstorage.HeadsUpdate) error {
	if f.fail {
		return errors.New("injected head update failure")
	}
	return f.HeadStorage.UpdateEntry(ctx, update)
}

func newTestStorage(t *testing.T) innerstorage.KeyValueStorage {
	storage, _ := newTestStorageWithFailingHeads(t)
	return storage
}

func newTestStorageWithFailingHeads(t *testing.T) (innerstorage.KeyValueStorage, *failingHeadStorage) {
	db, err := anystore.Open(ctx, filepath.Join(t.TempDir(), "store.db"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	heads, err := headstorage.New(ctx, db)
	require.NoError(t, err)
	failingHeads := &failingHeadStorage{HeadStorage: heads}
	storage, err := innerstorage.New(ctx, "kv.test", failingHeads, db)
	require.NoError(t, err)
	return storage, failingHeads
}

// TestSetFailureKeepsDiffConsistent asserts a failed Set leaves the in-memory
// diff matching the persisted state: the write tx rolls back, so the diff must
// not keep advertising heads that were never committed (peers would otherwise
// never re-send those values, and the advertised head-hash would diverge from
// storage until restart).
func TestSetFailureKeepsDiffConsistent(t *testing.T) {
	storage, failingHeads := newTestStorageWithFailingHeads(t)
	require.NoError(t, storage.Set(ctx, testKeyValue(0)))
	hashBefore := storage.Diff().Hash()

	failingHeads.fail = true
	kv := testKeyValue(1)
	require.Error(t, storage.Set(ctx, kv))
	failingHeads.fail = false

	_, err := storage.Diff().Element(kv.KeyPeerId)
	require.ErrorIs(t, err, ldiff.ErrElementNotFound, "diff must not advertise the rolled-back element")
	require.Equal(t, hashBefore, storage.Diff().Hash(), "diff hash must match persisted state after a failed Set")
	_, err = storage.GetKeyPeerId(ctx, kv.KeyPeerId)
	require.ErrorIs(t, err, anystore.ErrDocNotFound, "the rolled-back element must not be persisted")
}

// testKeyValue builds a KeyValue whose byte fields all have the same length but
// distinct contents, so a reused parse buffer overwrites them detectably.
func testKeyValue(i int) innerstorage.KeyValue {
	pattern := func(prefix byte) []byte {
		b := make([]byte, 32)
		for j := range b {
			b[j] = prefix + byte(i)
		}
		return b
	}
	return innerstorage.KeyValue{
		KeyPeerId:      fmt.Sprintf("key%d-peer", i),
		Key:            fmt.Sprintf("key%d", i),
		ReadKeyId:      "readKeyId",
		Identity:       "identity",
		PeerId:         "peer",
		TimestampMicro: int64(i + 1),
		Value: innerstorage.Value{
			Value:             pattern(0x10),
			PeerSignature:     pattern(0x40),
			IdentitySignature: pattern(0x70),
		},
	}
}

// TestIterateValuesKeyRowsAdjacent asserts the scan is id-ordered no matter
// the insertion order. Storage.Iterate builds its per-key callback groups
// from consecutive runs, so all rows of one key must come out adjacent —
// an insertion-ordered scan splits a key across groups whenever peers' row
// batches interleave (per-peer batch inserts put one key's rows far apart).
func TestIterateValuesKeyRowsAdjacent(t *testing.T) {
	storage := newTestStorage(t)
	const keys, peers = 16, 3
	for p := 0; p < peers; p++ {
		batch := make([]innerstorage.KeyValue, 0, keys)
		for k := 0; k < keys; k++ {
			kv := testKeyValue(k)
			kv.Key = fmt.Sprintf("key%02d", k)
			kv.PeerId = fmt.Sprintf("peer%d", p)
			kv.KeyPeerId = kv.Key + "-" + kv.PeerId
			batch = append(batch, kv)
		}
		require.NoError(t, storage.Set(ctx, batch...))
	}

	var ids, keysSeen []string
	require.NoError(t, storage.IterateValues(ctx, func(kv innerstorage.KeyValue) (bool, error) {
		ids = append(ids, kv.KeyPeerId)
		keysSeen = append(keysSeen, kv.Key)
		return true, nil
	}))
	require.Len(t, ids, keys*peers)
	require.True(t, sort.StringsAreSorted(ids), "scan must be id-ordered, got %v", ids)

	finished := map[string]bool{}
	var current string
	for _, key := range keysSeen {
		if key == current {
			continue
		}
		require.False(t, finished[key], "rows of %s split across non-adjacent runs", key)
		finished[current] = true
		current = key
	}
}

// TestIterateValuesReturnsOwnedMemory asserts the KeyValues handed to the
// iterator callback own their byte slices: retaining one across iterations must
// not let the next document's parse overwrite its contents.
func TestIterateValuesReturnsOwnedMemory(t *testing.T) {
	storage := newTestStorage(t)
	originals := map[string]innerstorage.KeyValue{}
	for i := 0; i < 3; i++ {
		kv := testKeyValue(i)
		originals[kv.KeyPeerId] = kv
		require.NoError(t, storage.Set(ctx, kv))
	}

	var collected []innerstorage.KeyValue
	err := storage.IterateValues(ctx, func(kv innerstorage.KeyValue) (bool, error) {
		collected = append(collected, kv)
		return true, nil
	})
	require.NoError(t, err)
	require.Len(t, collected, len(originals))
	for _, kv := range collected {
		want := originals[kv.KeyPeerId]
		require.Equal(t, want.Value.Value, kv.Value.Value, "value of %s must survive iteration", kv.KeyPeerId)
		require.Equal(t, want.Value.PeerSignature, kv.Value.PeerSignature, "peer signature of %s must survive iteration", kv.KeyPeerId)
		require.Equal(t, want.Value.IdentitySignature, kv.Value.IdentitySignature, "identity signature of %s must survive iteration", kv.KeyPeerId)
	}
}
