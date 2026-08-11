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

func mustSet(t *testing.T, st innerstorage.KeyValueStorage, kvs ...innerstorage.KeyValue) innerstorage.SetResult {
	t.Helper()
	res, err := st.Set(ctx, kvs...)
	require.NoError(t, err)
	return res
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
	mustSet(t, storage, testKeyValue(0))
	hashBefore := storage.Diff().Hash()

	failingHeads.fail = true
	kv := testKeyValue(1)
	_, setErr := storage.Set(ctx, kv)
	require.Error(t, setErr)
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
		mustSet(t, storage, batch...)
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
		mustSet(t, storage, kv)
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

// rowKV builds a normal row for watermark tests: id derived from key+peer,
// explicit timestamp.
func rowKV(key, peer string, ts int64) innerstorage.KeyValue {
	return innerstorage.KeyValue{
		KeyPeerId:      key + "-" + peer,
		Key:            key,
		ReadKeyId:      "readKeyId",
		Identity:       "identity",
		PeerId:         peer,
		TimestampMicro: ts,
		Value: innerstorage.Value{
			Value:             []byte("v-" + key + "-" + peer),
			PeerSignature:     []byte("ps"),
			IdentitySignature: []byte("is"),
		},
	}
}

// wmKV builds a deletion watermark row for the prefix.
func wmKV(prefix, peer string, ts int64) innerstorage.KeyValue {
	kv := rowKV(prefix, peer, ts)
	kv.DeletePrefix = prefix
	kv.Value.Value = nil
	return kv
}

func storedIds(t *testing.T, storage innerstorage.KeyValueStorage) []string {
	var ids []string
	require.NoError(t, storage.IterateValues(ctx, func(kv innerstorage.KeyValue) (bool, error) {
		ids = append(ids, kv.KeyPeerId)
		return true, nil
	}))
	sort.Strings(ids)
	return ids
}

// TestWatermarkDropsOlderRows: applying a watermark physically removes every
// row under the prefix older than it — document and diff element — while
// newer rows and rows outside the prefix survive.
func TestWatermarkDropsOlderRows(t *testing.T) {
	storage := newTestStorage(t)
	mustSet(t, storage,
		rowKV("read/sp1/obj1", "peerA", 10),
		rowKV("read/sp1/obj1", "peerB", 11),
		rowKV("read/sp1/obj2", "peerA", 12),
		rowKV("read/sp1/obj3", "peerA", 200), // newer than the watermark
		rowKV("read/sp2/obj1", "peerA", 13),  // outside the prefix
	)
	hashBefore := storage.Diff().Hash()

	wm := wmKV("read/sp1/", "peerA", 100)
	mustSet(t, storage, wm)

	require.Equal(t, []string{
		"read/sp1/-peerA", // the watermark row itself
		"read/sp1/obj3-peerA",
		"read/sp2/obj1-peerA",
	}, storedIds(t, storage))
	for _, dropped := range []string{"read/sp1/obj1-peerA", "read/sp1/obj1-peerB", "read/sp1/obj2-peerA"} {
		_, err := storage.Diff().Element(dropped)
		require.ErrorIs(t, err, ldiff.ErrElementNotFound, dropped)
	}
	_, err := storage.Diff().Element(wm.KeyPeerId)
	require.NoError(t, err, "watermark element must be advertised")
	require.NotEqual(t, hashBefore, storage.Diff().Hash())
}

// TestWatermarkRejectsLateArrivals: rows under the prefix older than an
// applied watermark are discarded on arrival; equal-or-newer rows apply.
func TestWatermarkRejectsLateArrivals(t *testing.T) {
	storage := newTestStorage(t)
	mustSet(t, storage, wmKV("read/sp1/", "peerA", 100))

	mustSet(t, storage, rowKV("read/sp1/obj1", "peerB", 99))
	_, err := storage.GetKeyPeerId(ctx, "read/sp1/obj1-peerB")
	require.ErrorIs(t, err, anystore.ErrDocNotFound, "older row must be rejected")

	mustSet(t, storage, rowKV("read/sp1/obj2", "peerB", 100))
	_, err = storage.GetKeyPeerId(ctx, "read/sp1/obj2-peerB")
	require.NoError(t, err, "equal-timestamp row is not covered")
}

// TestWatermarkSurvivesReopen: the watermark index is rebuilt from the stored
// watermark row, so rejection keeps working after a restart.
func TestWatermarkSurvivesReopen(t *testing.T) {
	db, err := anystore.Open(ctx, filepath.Join(t.TempDir(), "store.db"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	heads, err := headstorage.New(ctx, db)
	require.NoError(t, err)
	storage, err := innerstorage.New(ctx, "kv.test", heads, db)
	require.NoError(t, err)
	mustSet(t, storage, wmKV("read/sp1/", "peerA", 100))

	reopened, err := innerstorage.New(ctx, "kv.test", heads, db)
	require.NoError(t, err)
	mustSet(t, reopened, rowKV("read/sp1/obj1", "peerB", 99))
	_, err = reopened.GetKeyPeerId(ctx, "read/sp1/obj1-peerB")
	require.ErrorIs(t, err, anystore.ErrDocNotFound, "watermark must reject after reopen")
}

// TestWatermarkBatchOrderIndependent: a batch carrying both an old row and a
// watermark covering it converges to the same state in either order.
func TestWatermarkBatchOrderIndependent(t *testing.T) {
	for name, batch := range map[string][]innerstorage.KeyValue{
		"row first": {rowKV("read/sp1/obj1", "peerB", 50), wmKV("read/sp1/", "peerA", 100)},
		"wm first":  {wmKV("read/sp1/", "peerA", 100), rowKV("read/sp1/obj1", "peerB", 50)},
	} {
		t.Run(name, func(t *testing.T) {
			storage := newTestStorage(t)
			mustSet(t, storage, batch...)
			require.Equal(t, []string{"read/sp1/-peerA"}, storedIds(t, storage))
		})
	}
}

// TestWatermarkFailedTxRestoresDroppedElements: when the tx fails after a
// watermark dropped rows, the diff must re-advertise them — the documents
// come back with the rollback.
func TestWatermarkFailedTxRestoresDroppedElements(t *testing.T) {
	storage, failingHeads := newTestStorageWithFailingHeads(t)
	row := rowKV("read/sp1/obj1", "peerB", 50)
	mustSet(t, storage, row)
	hashBefore := storage.Diff().Hash()

	failingHeads.fail = true
	wm := wmKV("read/sp1/", "peerA", 100)
	_, setErr := storage.Set(ctx, wm)
	require.Error(t, setErr)
	failingHeads.fail = false

	_, err := storage.Diff().Element(row.KeyPeerId)
	require.NoError(t, err, "dropped element must be restored on rollback")
	_, err = storage.Diff().Element(wm.KeyPeerId)
	require.ErrorIs(t, err, ldiff.ErrElementNotFound, "rolled-back watermark must not be advertised")
	require.Equal(t, hashBefore, storage.Diff().Hash())
	_, err = storage.GetKeyPeerId(ctx, row.KeyPeerId)
	require.NoError(t, err, "row document must survive the rollback")

	// The rolled-back watermark must not keep rejecting: a fresh old row
	// still applies.
	mustSet(t, storage, rowKV("read/sp1/obj2", "peerB", 60))
	_, err = storage.GetKeyPeerId(ctx, "read/sp1/obj2-peerB")
	require.NoError(t, err)
}

// TestWatermarkKeyBoundary: the drop matches on the KEY, not the id — a
// prefix that extends past a full key into the "-"+peerId separator must not
// delete that key's rows, and must not reject its later arrivals.
func TestWatermarkKeyBoundary(t *testing.T) {
	storage := newTestStorage(t)
	mustSet(t, storage,
		rowKV("cnt", "1AB", 10),      // id "cnt-1AB" matches prefix "cnt-1"; key "cnt" does not
		rowKV("cnt-10", "peerA", 11), // key genuinely under the prefix
	)
	mustSet(t, storage, wmKV("cnt-1", "peerA", 100))

	require.Equal(t, []string{"cnt-1-peerA", "cnt-1AB"}, storedIds(t, storage),
		"key outside the prefix survives; key under it drops")
	mustSet(t, storage, rowKV("cnt", "1CD", 50))
	_, err := storage.GetKeyPeerId(ctx, "cnt-1CD")
	require.NoError(t, err, "late arrival for a key outside the prefix must apply")
}

// TestWatermarkFailedTxSameBatch: when a batch writes a row and a watermark
// that drops it and the tx then fails, the undo must leave no trace — a
// leftover advertised element would make the LWW pre-check discard the
// genuine row forever.
func TestWatermarkFailedTxSameBatch(t *testing.T) {
	storage, failingHeads := newTestStorageWithFailingHeads(t)
	hashBefore := storage.Diff().Hash()
	failingHeads.fail = true
	row := rowKV("read/sp1/obj1", "peerB", 50)
	wm := wmKV("read/sp1/", "peerA", 100)
	_, setErr := storage.Set(ctx, row, wm)
	require.Error(t, setErr)
	failingHeads.fail = false

	_, err := storage.Diff().Element(row.KeyPeerId)
	require.ErrorIs(t, err, ldiff.ErrElementNotFound, "rolled-back row must not be advertised")
	_, err = storage.Diff().Element(wm.KeyPeerId)
	require.ErrorIs(t, err, ldiff.ErrElementNotFound, "rolled-back watermark must not be advertised")
	require.Equal(t, hashBefore, storage.Diff().Hash())

	res := mustSet(t, storage, row)
	require.Len(t, res.Applied, 1, "the identical row must be storable after the rollback")
	_, err = storage.GetKeyPeerId(ctx, row.KeyPeerId)
	require.NoError(t, err)
}

// TestSetResultReportsAppliedAndDropped pins the SetResult contract: applied
// carries only LWW winners, droppedIds the watermark-removed rows.
func TestSetResultReportsAppliedAndDropped(t *testing.T) {
	storage := newTestStorage(t)
	res := mustSet(t, storage, rowKV("read/sp1/obj1", "peerA", 10))
	require.Len(t, res.Applied, 1)
	require.Empty(t, res.DroppedIds)

	res = mustSet(t, storage, rowKV("read/sp1/obj1", "peerA", 5)) // LWW loser
	require.Empty(t, res.Applied)

	res = mustSet(t, storage, wmKV("read/sp1/", "peerB", 100))
	require.Len(t, res.Applied, 1, "the watermark row itself applies")
	require.Equal(t, []string{"read/sp1/obj1-peerA"}, res.DroppedIds)
}
