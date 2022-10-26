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

type fixture struct {
	db *pogreb.DB
}

func newFixture(t *testing.T) *fixture {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	db, err := pogreb.Open(dir, nil)
	require.NoError(t, err)
	return &fixture{db: db}
}

func (fx *fixture) stop(t *testing.T) {
	require.NoError(t, fx.db.Close())
}

func TestTreeStorage_CreateTreeStorage(t *testing.T) {
	fx := newFixture(t)
	defer fx.stop(t)

	rootRawChange := &treechangeproto.RawTreeChangeWithId{RawChange: []byte("some"), Id: "rootId"}
	otherChange := &treechangeproto.RawTreeChangeWithId{RawChange: []byte("some other"), Id: "otherId"}
	changes := []*treechangeproto.RawTreeChangeWithId{rootRawChange, otherChange}
	payload := storage.TreeStorageCreatePayload{
		RootRawChange: rootRawChange,
		Changes:       changes,
		Heads:         []string{rootRawChange.Id},
	}
	store, err := createTreeStorage(fx.db, payload)
	require.NoError(t, err)
	require.Equal(t, payload.RootRawChange.Id, store.ID())

	root, err := store.Root()
	require.NoError(t, err)
	require.Equal(t, root, rootRawChange)

	for _, ch := range changes {
		dbCh, err := store.GetRawChange(context.Background(), ch.Id)
		require.NoError(t, err)
		require.Equal(t, ch, dbCh)
	}
}
