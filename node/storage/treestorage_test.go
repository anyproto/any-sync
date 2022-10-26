package storage

import (
	"context"
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func treeTestPayload() storage.TreeStorageCreatePayload {
	rootRawChange := &treechangeproto.RawTreeChangeWithId{RawChange: []byte("some"), Id: "rootId"}
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
	db  *pogreb.DB
}

func newFixture(t *testing.T) *fixture {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	return &fixture{dir: dir}
}

func (fx *fixture) open(t *testing.T) {
	var err error
	fx.db, err = pogreb.Open(fx.dir, nil)
	require.NoError(t, err)
}

func (fx *fixture) testPayloadInDB(t *testing.T, store storage.TreeStorage, payload storage.TreeStorageCreatePayload) {
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

func (fx *fixture) stop(t *testing.T) {
	require.NoError(t, fx.db.Close())
}

func TestTreeStorage(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	defer fx.stop(t)

	payload := treeTestPayload()
	store, err := createTreeStorage(fx.db, payload)
	require.NoError(t, err)
	fx.testPayloadInDB(t, store, payload)
}

func TestTreeStorage_Methods(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	payload := treeTestPayload()
	_, err := createTreeStorage(fx.db, payload)
	require.NoError(t, err)
	fx.stop(t)

	fx.open(t)
	defer fx.stop(t)
	store, err := newTreeStorage(fx.db, payload.RootRawChange.Id)
	require.NoError(t, err)
	fx.testPayloadInDB(t, store, payload)

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
