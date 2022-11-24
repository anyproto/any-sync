package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func treeTestPayload() storage.TreeStorageCreatePayload {
	rootRawChange := &treechangeproto.RawTreeChangeWithId{RawChange: []byte("some"), Id: "someRootId"}
	otherChange := &treechangeproto.RawTreeChangeWithId{RawChange: []byte("some other"), Id: "otherId"}
	changes := []*treechangeproto.RawTreeChangeWithId{rootRawChange, otherChange}
	return storage.TreeStorageCreatePayload{
		RootRawChange: rootRawChange,
		Changes:       changes,
		Heads:         []string{rootRawChange.Id},
	}
}

type fixture struct {
	dir string
	db  *badger.DB
}

func testTreePayload(t *testing.T, store storage.TreeStorage, payload storage.TreeStorageCreatePayload) {
	require.Equal(t, payload.RootRawChange.Id, store.ID())

	root, err := store.Root()
	require.NoError(t, err)
	require.Equal(t, root, payload.RootRawChange)

	heads, err := store.Heads()
	require.NoError(t, err)
	require.Equal(t, payload.Heads, heads)

	for _, ch := range payload.Changes {
		dbCh, err := store.GetRawChange(context.Background(), ch.Id)
		require.NoError(t, err)
		require.Equal(t, ch, dbCh)
	}
	return
}

func newFixture(t *testing.T) *fixture {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	return &fixture{dir: dir}
}

func (fx *fixture) open(t *testing.T) {
	var err error
	fx.db, err = badger.Open(badger.DefaultOptions(fx.dir))
	require.NoError(t, err)
}

func (fx *fixture) stop(t *testing.T) {
	require.NoError(t, fx.db.Close())
}

func (fx *fixture) testNoKeysExist(t *testing.T, spaceId, treeId string) {
	treeKeys := newTreeKeys(spaceId, treeId)

	var keys [][]byte
	err := fx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = treeKeys.RawChangePrefix()

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			keyCopy := make([]byte, 0, len(key))
			keyCopy = item.KeyCopy(key)
			keys = append(keys, keyCopy)
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 0, len(keys))

	err = fx.db.View(func(txn *badger.Txn) error {
		_, err = getTxn(txn, treeKeys.RootIdKey())
		require.Equal(t, err, badger.ErrKeyNotFound)

		_, err = getTxn(txn, treeKeys.HeadsKey())
		require.Equal(t, err, badger.ErrKeyNotFound)

		return nil
	})
}

func TestTreeStorage_Create(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	defer fx.stop(t)

	spaceId := "spaceId"
	payload := treeTestPayload()
	store, err := createTreeStorage(fx.db, spaceId, payload)
	require.NoError(t, err)
	testTreePayload(t, store, payload)

	t.Run("create same storage returns error", func(t *testing.T) {
		_, err := createTreeStorage(fx.db, spaceId, payload)
		require.Error(t, err)
	})
}

func TestTreeStorage_Methods(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	payload := treeTestPayload()
	spaceId := "spaceId"
	_, err := createTreeStorage(fx.db, spaceId, payload)
	require.NoError(t, err)
	fx.stop(t)

	fx.open(t)
	defer fx.stop(t)
	store, err := newTreeStorage(fx.db, spaceId, payload.RootRawChange.Id)
	require.NoError(t, err)
	testTreePayload(t, store, payload)

	t.Run("update heads", func(t *testing.T) {
		newHeads := []string{"a", "b"}
		require.NoError(t, store.SetHeads(newHeads))
		heads, err := store.Heads()
		require.NoError(t, err)
		require.Equal(t, newHeads, heads)
	})

	t.Run("add raw change, get change and has change", func(t *testing.T) {
		newChange := &treechangeproto.RawTreeChangeWithId{RawChange: []byte("ab"), Id: "newId"}
		require.NoError(t, store.AddRawChange(newChange))
		rawCh, err := store.GetRawChange(context.Background(), newChange.Id)
		require.NoError(t, err)
		require.Equal(t, newChange, rawCh)
		has, err := store.HasChange(context.Background(), newChange.Id)
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("get and has for unknown change", func(t *testing.T) {
		incorrectId := "incorrectId"
		_, err := store.GetRawChange(context.Background(), incorrectId)
		require.Error(t, err)
		has, err := store.HasChange(context.Background(), incorrectId)
		require.NoError(t, err)
		require.False(t, has)
	})
}

func TestTreeStorage_Delete(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	payload := treeTestPayload()
	spaceId := "spaceId"
	_, err := createTreeStorage(fx.db, spaceId, payload)
	require.NoError(t, err)
	fx.stop(t)

	fx.open(t)
	defer fx.stop(t)
	store, err := newTreeStorage(fx.db, spaceId, payload.RootRawChange.Id)
	require.NoError(t, err)
	testTreePayload(t, store, payload)

	t.Run("add raw change, get change and has change", func(t *testing.T) {
		newChange := &treechangeproto.RawTreeChangeWithId{RawChange: []byte("ab"), Id: "newId"}
		require.NoError(t, store.AddRawChange(newChange))

		err = store.Delete()
		require.NoError(t, err)

		_, err = newTreeStorage(fx.db, spaceId, payload.RootRawChange.Id)
		require.Equal(t, err, storage.ErrUnknownTreeId)

		fx.testNoKeysExist(t, spaceId, payload.RootRawChange.Id)
	})
}
