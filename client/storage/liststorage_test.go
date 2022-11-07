package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/require"
	"testing"
)

func testList(t *testing.T, store storage.ListStorage, root *aclrecordproto.RawACLRecordWithId, head string) {
	require.Equal(t, store.Id(), root.Id)

	aclRoot, err := store.Root()
	require.NoError(t, err)
	require.Equal(t, root, aclRoot)

	aclHead, err := store.Head()
	require.NoError(t, err)
	require.Equal(t, head, aclHead)
}

func TestListStorage(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	defer fx.stop(t)
	spaceId := "spaceId"
	aclRoot := &aclrecordproto.RawACLRecordWithId{Payload: []byte("root"), Id: "someRootId"}

	fx.db.Update(func(txn *badger.Txn) error {
		_, err := createListStorage(spaceId, fx.db, txn, aclRoot)
		require.NoError(t, err)
		return nil
	})

	var listStore storage.ListStorage
	fx.db.View(func(txn *badger.Txn) (err error) {
		listStore, err = newListStorage(spaceId, fx.db, txn)
		require.NoError(t, err)
		testList(t, listStore, aclRoot, aclRoot.Id)

		return nil
	})

	t.Run("create same storage returns no error", func(t *testing.T) {
		fx.db.View(func(txn *badger.Txn) error {
			// this is ok, because we only create new list storage when we create space storage
			listStore, err := createListStorage(spaceId, fx.db, txn, aclRoot)
			require.NoError(t, err)
			testList(t, listStore, aclRoot, aclRoot.Id)

			return nil
		})
	})

	t.Run("set head", func(t *testing.T) {
		head := "newHead"
		require.NoError(t, listStore.SetHead(head))
		aclHead, err := listStore.Head()
		require.NoError(t, err)
		require.Equal(t, head, aclHead)
	})

	t.Run("add raw record and get raw record", func(t *testing.T) {
		newRec := &aclrecordproto.RawACLRecordWithId{Payload: []byte("rec"), Id: "someRecId"}
		require.NoError(t, listStore.AddRawRecord(context.Background(), newRec))
		aclRec, err := listStore.GetRawRecord(context.Background(), newRec.Id)
		require.NoError(t, err)
		require.Equal(t, newRec, aclRec)
	})
}
