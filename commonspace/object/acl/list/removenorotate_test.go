package list

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func (fx *childFixture) removeNoRotateByParent(t *testing.T, identities ...crypto.PubKey) *consensusproto.RawRecord {
	rec, err := fx.parentBuilder(t).BuildAccountRemoveNoRotate(identities)
	require.NoError(t, err)
	return rec
}

// addMember adds one more account to the child and returns its keys
func (fx *childFixture) addMember(t *testing.T, perms AclPermissions) *accountdata.AccountKeys {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	rec, err := fx.ownerAcl.RecordBuilder().BuildAccountsAdd(AccountsAddPayload{Additions: []AccountAdd{{
		Identity:    keys.SignKey.GetPublic(),
		Permissions: perms,
		Metadata:    mockMetadata,
	}}})
	require.NoError(t, err)
	fx.addRec(t, listtest.WrapAclRecord(rec))
	return keys
}

func TestAccountRemoveNoRotate_Admission(t *testing.T) {
	removeMembers := ParentAuthority{RemoveMembers: true}

	t.Run("refused without an authorizer", func(t *testing.T) {
		fx := newChildFixture(t)
		rec := fx.removeNoRotateByParent(t, fx.account())
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(rec, nil), ErrInsufficientPermissions)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, nil, nil), ErrInsufficientPermissions)
	})

	t.Run("refused without the authority to remove members", func(t *testing.T) {
		fx := newChildFixture(t)
		rec := fx.removeNoRotateByParent(t, fx.account())
		err := fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, fx.authorize(ParentAuthority{ChangeOwner: true}), nil)
		require.ErrorIs(t, err, ErrInsufficientPermissions)
	})

	t.Run("no role in the child gives the authority", func(t *testing.T) {
		fx := newChildFixture(t)
		rec, err := fx.ownerAcl.RecordBuilder().BuildAccountRemoveNoRotate([]crypto.PubKey{fx.account()})
		require.NoError(t, err)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(rec, nil), ErrInsufficientPermissions)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, fx.authorize(removeMembers), nil), ErrInsufficientPermissions)
	})

	t.Run("accepted with the authority", func(t *testing.T) {
		fx := newChildFixture(t)
		readKeyId := fx.ownerAcl.AclState().CurrentReadKeyId()
		rec := fx.removeNoRotateByParent(t, fx.account())
		var checked bool
		err := fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, fx.authorize(removeMembers), func(st *AclState) error {
			checked = true
			require.True(t, st.Permissions(fx.account()).NoPermissions())
			require.True(t, st.RotationPending())
			require.Equal(t, readKeyId, st.CurrentReadKeyId())
			return nil
		})
		require.NoError(t, err)
		require.True(t, checked)
		// validation changes nothing
		require.True(t, fx.ownerAcl.AclState().Permissions(fx.account()).CanWrite())
		require.False(t, fx.ownerAcl.AclState().RotationPending())
	})

	t.Run("the authorizer's error is returned", func(t *testing.T) {
		fx := newChildFixture(t)
		rec := fx.removeNoRotateByParent(t, fx.account())
		lookupErr := errors.New("parent acl is not available")
		err := fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, func(crypto.PubKey) (ParentAuthority, error) {
			return removeMembers, lookupErr
		}, nil)
		require.ErrorIs(t, err, lookupErr)
	})
}

// The authority covers who writes the record, not whom it can remove.
func TestAccountRemoveNoRotate_Rules(t *testing.T) {
	fx := newChildFixture(t)
	stranger, err := accountdata.NewRandom()
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		identities []crypto.PubKey
		err        error
	}{
		"the owner":             {[]crypto.PubKey{fx.owner()}, ErrInsufficientPermissions},
		"the owner in a batch":  {[]crypto.PubKey{fx.account(), fx.owner()}, ErrInsufficientPermissions},
		"not a member":          {[]crypto.PubKey{stranger.SignKey.GetPublic()}, ErrNoSuchAccount},
		"nobody":                {nil, ErrIncorrectNumberOfAccounts},
		"the same member twice": {[]crypto.PubKey{fx.account(), fx.account()}, ErrDuplicateAccounts},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := fx.parentBuilder(t).BuildAccountRemoveNoRotate(tc.identities)
			require.ErrorIs(t, err, tc.err)
		})
	}

	t.Run("the author", func(t *testing.T) {
		_, err := fx.accountAcl.RecordBuilder().BuildAccountRemoveNoRotate([]crypto.PubKey{fx.account()})
		require.ErrorIs(t, err, ErrInsufficientPermissions)
	})
}

func TestAccountRemoveNoRotate_RotationPending(t *testing.T) {
	fx := newChildFixture(t)
	writerKeys := fx.addMember(t, AclPermissionsWriter)
	prevHead := fx.ownerAcl.Head().Id
	readKeyId := fx.ownerAcl.AclState().CurrentReadKeyId()
	require.False(t, fx.ownerAcl.AclState().RotationPending())

	removal := listtest.WrapAclRecord(fx.removeNoRotateByParent(t, fx.account()))
	fx.addRec(t, removal)

	for _, st := range []*AclState{fx.ownerAcl.AclState(), fx.accountAcl.AclState()} {
		// the member has no permissions from this record on
		require.True(t, st.Permissions(fx.account()).NoPermissions())
		before, err := st.PermissionsAtRecord(prevHead, fx.account())
		require.NoError(t, err)
		require.Equal(t, AclPermissionsWriter, before)
		after, err := st.PermissionsAtRecord(removal.Id, fx.account())
		require.NoError(t, err)
		require.True(t, after.NoPermissions())

		// the read key is the one the removed member holds
		require.True(t, st.RotationPending())
		require.True(t, st.Copy().RotationPending())
		require.Equal(t, readKeyId, st.CurrentReadKeyId())
		require.True(t, st.Permissions(fx.parent()).NoPermissions())
	}

	t.Run("a Writer cannot change the read key", func(t *testing.T) {
		writerAcl := fx.rebuild(t, writerKeys, recordverifier.NewValidateFull())
		require.True(t, writerAcl.AclState().RotationPending())
		_, err := writerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
		require.ErrorIs(t, err, ErrInsufficientPermissions)

		// the owner's change, signed by the Writer
		rotation, err := fx.ownerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
		require.NoError(t, err)
		var payload consensusproto.Record
		require.NoError(t, payload.UnmarshalVT(rotation.Payload))
		raw, _ := signAclRecord(t, writerKeys, payload.PrevId, payload.Data)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(raw, nil), ErrInsufficientPermissions)
	})

	t.Run("the owner's read key change ends it", func(t *testing.T) {
		rotation, err := fx.ownerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
		require.NoError(t, err)
		require.NoError(t, fx.ownerAcl.ValidateRawRecord(rotation, nil))
		require.NoError(t, fx.ownerAcl.AddRawRecord(listtest.WrapAclRecord(rotation)))

		st := fx.ownerAcl.AclState()
		require.False(t, st.RotationPending())
		require.False(t, st.Copy().RotationPending())
		require.NotEqual(t, readKeyId, st.CurrentReadKeyId())
	})
}

func TestAccountRemoveNoRotate_RemovalWithRotationEndsPending(t *testing.T) {
	fx := newChildFixture(t)
	writerKeys := fx.addMember(t, AclPermissionsWriter)
	fx.addRec(t, listtest.WrapAclRecord(fx.removeNoRotateByParent(t, fx.account())))
	require.True(t, fx.ownerAcl.AclState().RotationPending())

	remove, err := fx.ownerAcl.RecordBuilder().BuildAccountRemove(AccountRemovePayload{
		Identities: []crypto.PubKey{writerKeys.SignKey.GetPublic()},
		Change:     newTestReadKeyChangePayload(),
	})
	require.NoError(t, err)
	require.NoError(t, fx.ownerAcl.AddRawRecord(listtest.WrapAclRecord(remove)))
	require.False(t, fx.ownerAcl.AclState().RotationPending())
}

// A removal in the log was accepted by the coordinator, and the acl of a child stores nothing about its
// parent. Rebuilding the log applies it without asking who the author is, and so does a list that
// ingests the accepted log. A list that validates in full what it ingests refuses the removal.
func TestAccountRemoveNoRotate_Replay(t *testing.T) {
	fx := newChildFixture(t)
	prevHead := fx.ownerAcl.Head().Id
	removal := listtest.WrapAclRecord(fx.removeNoRotateByParent(t, fx.account()))

	validating := fx.rebuild(t, fx.ownerKeys, recordverifier.NewValidateFull())
	require.ErrorIs(t, validating.AddRawRecord(removal), ErrInsufficientPermissions)
	require.Equal(t, prevHead, validating.Head().Id)
	require.True(t, validating.AclState().Permissions(fx.account()).CanWrite())

	// the lists of the fixture ingest the accepted log
	fx.addRec(t, removal)

	verifiers := map[string]recordverifier.AcceptorVerifier{
		"validating":     recordverifier.NewValidateFull(),
		"accepted log":   recordverifier.NewValidateAcceptedLog(),
		"non-validating": noValidateVerifier{},
	}
	for name, verifier := range verifiers {
		t.Run(name, func(t *testing.T) {
			for _, keys := range []*accountdata.AccountKeys{fx.ownerKeys, fx.accountKeys, fx.parentKeys} {
				st := fx.rebuild(t, keys, verifier).AclState()
				require.Equal(t, removal.Id, st.LastRecordId())
				require.True(t, st.Permissions(fx.account()).NoPermissions())
				require.True(t, st.RotationPending())
			}

			// a fresh list ingests the whole log, as the storage migration does with the accepted log
			ctx := context.Background()
			store := createStore(ctx, t)
			headStorage, err := headstorage.New(ctx, store)
			require.NoError(t, err)
			storage, err := CreateStorage(ctx, fx.ownerAcl.Root(), headStorage, store)
			require.NoError(t, err)
			migrated, err := BuildAclListWithIdentity(fx.ownerKeys, storage, verifier)
			require.NoError(t, err)
			var records []*consensusproto.RawRecordWithId
			for _, rec := range fx.ownerAcl.Records()[1:] {
				raw, err := fx.ownerAcl.storage.Get(ctx, rec.Id)
				require.NoError(t, err)
				records = append(records, raw.RawRecordWithId())
			}
			err = migrated.AddRawRecords(records)
			if name == "validating" {
				require.ErrorIs(t, err, ErrInsufficientPermissions)
				require.Equal(t, prevHead, migrated.AclState().LastRecordId())
				return
			}
			require.NoError(t, err)
			require.Equal(t, removal.Id, migrated.AclState().LastRecordId())
			require.True(t, migrated.AclState().RotationPending())
		})
	}

	// the owner builds the read key change on top of the removal, and the log with both rebuilds
	rotation, err := fx.ownerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
	require.NoError(t, err)
	require.NoError(t, fx.ownerAcl.AddRawRecord(listtest.WrapAclRecord(rotation)))
	for name, verifier := range verifiers {
		t.Run("rotated/"+name, func(t *testing.T) {
			st := fx.rebuild(t, fx.ownerKeys, verifier).AclState()
			require.False(t, st.RotationPending())
			require.True(t, st.Permissions(fx.account()).NoPermissions())
		})
	}
}
