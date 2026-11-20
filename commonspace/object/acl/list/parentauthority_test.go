package list

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

// childFixture is the acl of a child space: ownerKeys own it, accountKeys are a Writer in it, and
// parentKeys belong to an account of the parent space that is not a member of the child. Its lists
// ingest the log the network accepted.
type childFixture struct {
	*aclFixture
	parentKeys *accountdata.AccountKeys
}

func newChildFixture(t *testing.T) *childFixture {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissionsWriter)
	parentKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	fx.ownerAcl = acceptedLogList(t, fx.ownerKeys, fx.ownerAcl.storage)
	fx.accountAcl = acceptedLogList(t, fx.accountKeys, fx.accountAcl.storage)
	return &childFixture{aclFixture: fx, parentKeys: parentKeys}
}

func acceptedLogList(t *testing.T, keys *accountdata.AccountKeys, storage Storage) *aclList {
	l, err := BuildAclListWithIdentity(keys, storage, recordverifier.NewValidateAcceptedLog())
	require.NoError(t, err)
	return l.(*aclList)
}

func (fx *childFixture) owner() crypto.PubKey   { return fx.ownerKeys.SignKey.GetPublic() }
func (fx *childFixture) account() crypto.PubKey { return fx.accountKeys.SignKey.GetPublic() }
func (fx *childFixture) parent() crypto.PubKey  { return fx.parentKeys.SignKey.GetPublic() }

// parentBuilder builds records as the account of the parent space does: on a list made from the
// records of the child, without a read key of it.
func (fx *childFixture) parentBuilder(t *testing.T) AclRecordBuilder {
	return fx.rebuild(t, fx.parentKeys, noValidateVerifier{}).RecordBuilder()
}

func (fx *childFixture) rebuild(t *testing.T, keys *accountdata.AccountKeys, verifier recordverifier.AcceptorVerifier) AclList {
	l, err := BuildAclListWithIdentity(keys, fx.ownerAcl.storage, verifier)
	require.NoError(t, err)
	return l
}

func (fx *childFixture) ownershipChangeByParent(t *testing.T, newOwner crypto.PubKey, oldOwnerPermissions AclPermissions) *consensusproto.RawRecord {
	rec, err := fx.parentBuilder(t).BuildOwnershipChangeByParent(OwnershipChangePayload{
		NewOwner:            newOwner,
		OldOwnerPermissions: oldOwnerPermissions,
	})
	require.NoError(t, err)
	return rec
}

// authorize gives authority to parentKeys only
func (fx *childFixture) authorize(authority ParentAuthority) ParentAuthorizer {
	return func(author crypto.PubKey) (ParentAuthority, error) {
		if author.Equals(fx.parent()) {
			return authority, nil
		}
		return ParentAuthority{}, nil
	}
}

func neverAuthorize(t *testing.T) ParentAuthorizer {
	return func(crypto.PubKey) (ParentAuthority, error) {
		t.Fatal("the authorizer must not be asked")
		return ParentAuthority{}, nil
	}
}

func TestOwnershipChangeByParent_Admission(t *testing.T) {
	t.Run("refused without an authorizer", func(t *testing.T) {
		fx := newChildFixture(t)
		rec := fx.ownershipChangeByParent(t, fx.account(), AclPermissionsAdmin)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(rec, nil), ErrInsufficientPermissions)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, nil, nil), ErrInsufficientPermissions)
	})

	t.Run("refused without the authority to change the owner", func(t *testing.T) {
		fx := newChildFixture(t)
		rec := fx.ownershipChangeByParent(t, fx.account(), AclPermissionsAdmin)
		err := fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, fx.authorize(ParentAuthority{RemoveMembers: true}), nil)
		require.ErrorIs(t, err, ErrInsufficientPermissions)
	})

	t.Run("accepted with the authority", func(t *testing.T) {
		fx := newChildFixture(t)
		rec := fx.ownershipChangeByParent(t, fx.account(), AclPermissionsAdmin)
		var checked bool
		err := fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, fx.authorize(ParentAuthority{ChangeOwner: true}), func(st *AclState) error {
			checked = true
			require.True(t, st.Permissions(fx.account()).IsOwner())
			require.Equal(t, AclPermissionsAdmin, st.Permissions(fx.owner()))
			require.True(t, st.Permissions(fx.parent()).NoPermissions())
			return nil
		})
		require.NoError(t, err)
		require.True(t, checked)
		// validation changes nothing
		require.True(t, fx.ownerAcl.AclState().Permissions(fx.owner()).IsOwner())
	})

	t.Run("the authorizer's error is returned", func(t *testing.T) {
		fx := newChildFixture(t)
		rec := fx.ownershipChangeByParent(t, fx.account(), AclPermissionsAdmin)
		lookupErr := errors.New("parent acl is not available")
		err := fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, func(crypto.PubKey) (ParentAuthority, error) {
			return ParentAuthority{ChangeOwner: true}, lookupErr
		}, nil)
		require.ErrorIs(t, err, lookupErr)
	})

	t.Run("the owner of the child needs no authority", func(t *testing.T) {
		fx := newChildFixture(t)
		rec, err := fx.ownerAcl.RecordBuilder().BuildOwnershipChange(OwnershipChangePayload{
			NewOwner:            fx.account(),
			OldOwnerPermissions: AclPermissionsWriter,
		})
		require.NoError(t, err)
		require.NoError(t, fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, neverAuthorize(t), nil))
	})

	t.Run("a member who is not the owner needs the authority", func(t *testing.T) {
		fx := newChildFixture(t)
		_, err := fx.accountAcl.RecordBuilder().BuildOwnershipChange(OwnershipChangePayload{
			NewOwner:            fx.account(),
			OldOwnerPermissions: AclPermissionsWriter,
		})
		require.ErrorIs(t, err, ErrInsufficientPermissions)

		rec, err := fx.accountAcl.RecordBuilder().BuildOwnershipChangeByParent(OwnershipChangePayload{
			NewOwner:            fx.account(),
			OldOwnerPermissions: AclPermissionsWriter,
		})
		require.NoError(t, err)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(rec, nil), ErrInsufficientPermissions)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecordWithAuthorizer(rec, fx.authorize(ParentAuthority{ChangeOwner: true}), nil), ErrInsufficientPermissions)
	})
}

// The authority covers who writes the record, not what a valid ownership change is.
func TestOwnershipChangeByParent_Rules(t *testing.T) {
	fx := newChildFixture(t)
	stranger, err := accountdata.NewRandom()
	require.NoError(t, err)
	builder := fx.parentBuilder(t)

	for name, tc := range map[string]struct {
		payload OwnershipChangePayload
		err     error
	}{
		"the new owner is not a member":         {OwnershipChangePayload{NewOwner: stranger.SignKey.GetPublic(), OldOwnerPermissions: AclPermissionsAdmin}, ErrNoSuchAccount},
		"the new owner is the owner":            {OwnershipChangePayload{NewOwner: fx.owner(), OldOwnerPermissions: AclPermissionsAdmin}, ErrInsufficientPermissions},
		"the previous owner stays the owner":    {OwnershipChangePayload{NewOwner: fx.account(), OldOwnerPermissions: AclPermissionsOwner}, ErrInsufficientPermissions},
		"the previous owner is left no role":    {OwnershipChangePayload{NewOwner: fx.account(), OldOwnerPermissions: AclPermissionsNone}, ErrInsufficientPermissions},
		"the new owner is the parent's account": {OwnershipChangePayload{NewOwner: fx.parent(), OldOwnerPermissions: AclPermissionsAdmin}, ErrNoSuchAccount},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := builder.BuildOwnershipChangeByParent(tc.payload)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

// A record in the log was accepted by the coordinator, and the acl of a child stores nothing about its
// parent. Rebuilding the log applies the change without asking who the author is, and so does a list
// that ingests the accepted log. A list that validates in full what it ingests refuses the record.
func TestOwnershipChangeByParent_Replay(t *testing.T) {
	fx := newChildFixture(t)
	prevHead := fx.ownerAcl.Head().Id
	rec := listtest.WrapAclRecord(fx.ownershipChangeByParent(t, fx.account(), AclPermissionsReader))

	validating := fx.rebuild(t, fx.ownerKeys, recordverifier.NewValidateFull())
	require.ErrorIs(t, validating.AddRawRecord(rec), ErrInsufficientPermissions)
	require.Equal(t, prevHead, validating.Head().Id)
	require.True(t, validating.AclState().Permissions(fx.owner()).IsOwner())

	check := func(t *testing.T, st *AclState) {
		require.Equal(t, rec.Id, st.LastRecordId())
		newOwner, err := st.OwnerPubKey()
		require.NoError(t, err)
		require.True(t, newOwner.Equals(fx.account()))
		// the previous owner is the owner the state had, not the author of the record
		require.Equal(t, AclPermissionsReader, st.Permissions(fx.owner()))
		require.True(t, st.Permissions(fx.parent()).NoPermissions())

		before, err := st.PermissionsAtRecord(prevHead, fx.owner())
		require.NoError(t, err)
		require.True(t, before.IsOwner())
		after, err := st.PermissionsAtRecord(rec.Id, fx.owner())
		require.NoError(t, err)
		require.Equal(t, AclPermissionsReader, after)
	}

	// the lists of the fixture ingest the accepted log
	fx.addRec(t, rec)
	check(t, fx.ownerAcl.AclState())
	check(t, fx.accountAcl.AclState())

	for name, verifier := range map[string]recordverifier.AcceptorVerifier{
		"validating":     recordverifier.NewValidateFull(),
		"accepted log":   recordverifier.NewValidateAcceptedLog(),
		"non-validating": noValidateVerifier{},
	} {
		t.Run("rebuild/"+name, func(t *testing.T) {
			for _, keys := range []*accountdata.AccountKeys{fx.ownerKeys, fx.accountKeys, fx.parentKeys} {
				check(t, fx.rebuild(t, keys, verifier).AclState())
			}
		})
	}

	// the new owner acts as the owner, the previous one no longer does
	_, err := fx.accountAcl.RecordBuilder().BuildSpaceOptionsChange(nil)
	require.NoError(t, err)
	_, err = fx.ownerAcl.RecordBuilder().BuildSpaceOptionsChange(nil)
	require.ErrorIs(t, err, ErrInsufficientPermissions)
}
