package list

import (
	"context"
	"errors"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

// failCommitStore fails the commit of every write transaction while failCommit is set.
type failCommitStore struct {
	anystore.DB
	failCommit bool
}

func (s *failCommitStore) WriteTx(ctx context.Context) (anystore.WriteTx, error) {
	tx, err := s.DB.WriteTx(ctx)
	if err != nil || !s.failCommit {
		return tx, err
	}
	return failCommitTx{tx}, nil
}

type failCommitTx struct {
	anystore.WriteTx
}

var errCommit = errors.New("commit failed")

func (tx failCommitTx) Commit() error {
	_ = tx.WriteTx.Rollback()
	return errCommit
}

// refusingStorage fails AddAll while err is set.
type refusingStorage struct {
	Storage
	err error
}

func (s *refusingStorage) AddAll(ctx context.Context, records []StorageRecord) error {
	if s.err != nil {
		return s.err
	}
	return s.Storage.AddAll(ctx, records)
}

func TestAclList_RefusedRecordDoesNotBecomePrevious(t *testing.T) {
	fx := newFixture(t)
	acl := fx.ownerAcl
	st := &refusingStorage{Storage: acl.storage}
	acl.storage = st
	head := acl.Head().Id

	refused, err := acl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	st.err = errors.New("error saving")
	require.ErrorIs(t, acl.AddRawRecord(listtest.WrapAclRecord(refused.InviteRec)), st.err)
	require.Equal(t, head, acl.Head().Id)
	require.Empty(t, acl.AclState().Invites())

	st.err = nil
	inv, err := acl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	rec := listtest.WrapAclRecord(inv.InviteRec)
	require.NoError(t, acl.AddRawRecord(rec))
	require.Equal(t, rec.Id, acl.Head().Id)
	stored, err := st.Get(context.Background(), rec.Id)
	require.NoError(t, err)
	require.Equal(t, head, stored.PrevId)
	require.Len(t, acl.AclState().Invites(), 1)
}

func TestAclList_FailedCommitIsARefusedRecord(t *testing.T) {
	ctx := context.Background()
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	store := &failCommitStore{DB: createStore(ctx, t)}
	built, err := newDerivedAclWithStoreProvider("spaceId", keys, []byte("metadata"), func(root *consensusproto.RawRecordWithId) (Storage, error) {
		headStorage, err := headstorage.New(ctx, store)
		require.NoError(t, err)
		return CreateStorage(ctx, root, headStorage, store)
	})
	require.NoError(t, err)
	acl := built.(*aclList)
	head := acl.Head().Id

	refused, err := acl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	store.failCommit = true
	require.ErrorIs(t, acl.AddRawRecord(listtest.WrapAclRecord(refused.InviteRec)), errCommit)
	require.Equal(t, head, acl.Head().Id)

	store.failCommit = false
	inv, err := acl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	rec := listtest.WrapAclRecord(inv.InviteRec)
	require.NoError(t, acl.AddRawRecord(rec))
	stored, err := acl.storage.Get(ctx, rec.Id)
	require.NoError(t, err)
	require.Equal(t, head, stored.PrevId)
}
