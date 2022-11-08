package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
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

func TestListStorage_Create(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	defer fx.stop(t)

	aclRoot := &aclrecordproto.RawACLRecordWithId{Payload: []byte("root"), Id: "someRootId"}
	listStore, err := createListStorage(fx.db, aclRoot)
	require.NoError(t, err)
	testList(t, listStore, aclRoot, aclRoot.Id)

	t.Run("create same list storage returns nil", func(t *testing.T) {
		// this is ok, because we only create new list storage when we create space storage
		listStore, err := createListStorage(fx.db, aclRoot)
		require.NoError(t, err)
		testList(t, listStore, aclRoot, aclRoot.Id)
	})
}

func TestListStorage_Methods(t *testing.T) {
	fx := newFixture(t)
	fx.open(t)
	aclRoot := &aclrecordproto.RawACLRecordWithId{Payload: []byte("root"), Id: "someRootId"}
	_, err := createListStorage(fx.db, aclRoot)
	require.NoError(t, err)
	fx.stop(t)

	fx.open(t)
	defer fx.stop(t)
	listStore, err := newListStorage(fx.db)
	require.NoError(t, err)
	testList(t, listStore, aclRoot, aclRoot.Id)

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
