package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func spaceTestPayload() spacestorage.SpaceStorageCreatePayload {
	header := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: []byte("header"),
		Id:        "headerId",
	}
	aclRoot := &aclrecordproto.RawACLRecordWithId{
		Payload: []byte("aclRoot"),
		Id:      "aclRootId",
	}
	return spacestorage.SpaceStorageCreatePayload{
		AclWithId:         aclRoot,
		SpaceHeaderWithId: header,
	}
}

func testSpace(t *testing.T, store spacestorage.SpaceStorage, payload spacestorage.SpaceStorageCreatePayload) {
	header, err := store.SpaceHeader()
	require.NoError(t, err)
	require.Equal(t, payload.SpaceHeaderWithId, header)

	aclStorage, err := store.ACLStorage()
	require.NoError(t, err)
	testList(t, aclStorage, payload.AclWithId, payload.AclWithId.Id)
}

func TestSpaceStorage_Create(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	defer fx.stop(t)

	payload := spaceTestPayload()
	store, err := createSpaceStorage(fx.db, payload)
	require.NoError(t, err)

	testSpace(t, store, payload)
	require.NoError(t, store.Close())

	t.Run("create same storage returns error", func(t *testing.T) {
		_, err := createSpaceStorage(fx.db, payload)
		require.Error(t, err)
	})
}

func TestSpaceStorage_NewAndCreateTree(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)

	payload := spaceTestPayload()
	store, err := createSpaceStorage(fx.db, payload)
	require.NoError(t, err)
	require.NoError(t, store.Close())
	fx.stop(t)

	fx.open(t)
	defer fx.stop(t)
	store, err = newSpaceStorage(fx.db, payload.SpaceHeaderWithId.Id)
	require.NoError(t, err)
	testSpace(t, store, payload)

	t.Run("create tree and get tree", func(t *testing.T) {
		payload := treeTestPayload()
		treeStore, err := store.CreateTreeStorage(payload)
		require.NoError(t, err)
		testTreePayload(t, treeStore, payload)

		otherStore, err := store.TreeStorage(payload.RootRawChange.Id)
		require.NoError(t, err)
		testTreePayload(t, otherStore, payload)
	})
}

func TestSpaceStorage_StoredIds(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	defer fx.stop(t)

	payload := spaceTestPayload()
	store, err := createSpaceStorage(fx.db, payload)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	n := 5
	var ids []string
	for i := 0; i < n; i++ {
		treePayload := treeTestPayload()
		treePayload.RootRawChange.Id += strconv.Itoa(i)
		ids = append(ids, treePayload.RootRawChange.Id)
		_, err := store.CreateTreeStorage(treePayload)
		require.NoError(t, err)
	}

	storedIds, err := store.StoredIds()
	require.NoError(t, err)
	require.Equal(t, ids, storedIds)
}
