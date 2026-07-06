package list

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

// childAclFixture is a child space acl shared by its owner and one writer, with the
// parent owner (legalOwner) holding no permissions in it
type childAclFixture struct {
	legalOwnerKeys *accountdata.AccountKeys
	ownerKeys      *accountdata.AccountKeys
	writerKeys     *accountdata.AccountKeys
	ownerAcl       AclList
	writerAcl      AclList
}

func addToAll(t *testing.T, rec *consensusproto.RawRecordWithId, acls ...AclList) {
	for _, acl := range acls {
		require.NoError(t, acl.AddRawRecord(rec))
	}
}

func newChildAclFixture(t *testing.T, options *aclrecordproto.AclSpaceOptions) *childAclFixture {
	legalOwnerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	writerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)

	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	root, err := newTestAclRecordBuilder(ownerKeys).BuildRoot(RootContent{
		PrivKey:       ownerKeys.SignKey,
		MasterKey:     masterKey,
		Change:        newTestReadKeyChangePayload(),
		Metadata:      []byte("m"),
		Options:       options,
		ParentSpaceId: "parent.id",
		LegalOwner:    legalOwnerKeys.SignKey.GetPublic(),
	})
	require.NoError(t, err)

	storage, err := NewInMemoryStorage(root.Id, []*consensusproto.RawRecordWithId{root})
	require.NoError(t, err)
	ownerAcl, err := BuildAclListWithIdentity(ownerKeys, storage, recordverifier.NewValidateFull())
	require.NoError(t, err)
	writerAcl, err := BuildAclListWithIdentity(writerKeys, storage, recordverifier.NewValidateFull())
	require.NoError(t, err)

	fx := &childAclFixture{
		legalOwnerKeys: legalOwnerKeys,
		ownerKeys:      ownerKeys,
		writerKeys:     writerKeys,
		ownerAcl:       ownerAcl,
		writerAcl:      writerAcl,
	}

	// the owner adds the writer directly (docs/15 direct-add path)
	add, err := ownerAcl.RecordBuilder().BuildAccountsAdd(AccountsAddPayload{
		Additions: []AccountAdd{{
			Identity:    writerKeys.SignKey.GetPublic(),
			Permissions: AclPermissionsWriter,
			Metadata:    []byte("wm"),
		}},
	})
	require.NoError(t, err)
	addToAll(t, listtest.WrapAclRecord(add), ownerAcl, writerAcl)
	return fx
}

func (fx *childAclFixture) removeNoRotateRecord(t *testing.T, signer *accountdata.AccountKeys, targets ...crypto.PubKey) *consensusproto.RawRecordWithId {
	var identities [][]byte
	for _, target := range targets {
		protoIdentity, err := target.Marshall()
		require.NoError(t, err)
		identities = append(identities, protoIdentity)
	}
	content := &aclrecordproto.AclContentValue{
		Value: &aclrecordproto.AclContentValue_AccountRemoveNoRotate{
			AccountRemoveNoRotate: &aclrecordproto.AclAccountRemoveNoRotate{Identities: identities},
		},
	}
	return buildAclRecordSignedBy(t, fx.ownerAcl.Head().Id, signer, content)
}

func TestKeylessRemove_LegalOwnerRemovesWriter(t *testing.T) {
	fx := newChildAclFixture(t, nil)
	readKeyIdBefore := fx.ownerAcl.AclState().CurrentReadKeyId()

	rec := fx.removeNoRotateRecord(t, fx.legalOwnerKeys, fx.writerKeys.SignKey.GetPublic())
	addToAll(t, rec, fx.ownerAcl, fx.writerAcl)

	st := fx.ownerAcl.AclState()
	require.True(t, st.Permissions(fx.writerKeys.SignKey.GetPublic()).NoPermissions())
	require.True(t, st.HasPendingKeylessRemovals())
	require.Len(t, st.PendingKeylessRemovals(), 1)
	require.True(t, st.PendingKeylessRemovals()[0].Equals(fx.writerKeys.SignKey.GetPublic()))
	// no rotation happened
	require.Equal(t, readKeyIdBefore, st.CurrentReadKeyId())

	// a key-holding admin (the owner) completes the cut-off with a standard rotation
	rotation, err := fx.ownerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
	require.NoError(t, err)
	require.NoError(t, fx.ownerAcl.AddRawRecord(listtest.WrapAclRecord(rotation)))

	st = fx.ownerAcl.AclState()
	require.False(t, st.HasPendingKeylessRemovals())
	require.NotEqual(t, readKeyIdBefore, st.CurrentReadKeyId())
}

func TestKeylessRemove_Rejections(t *testing.T) {
	t.Run("only the legal owner may author", func(t *testing.T) {
		fx := newChildAclFixture(t, nil)
		stranger, err := accountdata.NewRandom()
		require.NoError(t, err)
		rec := fx.removeNoRotateRecord(t, stranger, fx.writerKeys.SignKey.GetPublic())
		require.ErrorIs(t, fx.ownerAcl.AddRawRecord(rec), ErrInsufficientPermissions)

		// even the child owner cannot use the keyless record — it must rotate via AclAccountRemove
		rec = fx.removeNoRotateRecord(t, fx.ownerKeys, fx.writerKeys.SignKey.GetPublic())
		require.ErrorIs(t, fx.ownerAcl.AddRawRecord(rec), ErrInsufficientPermissions)
	})

	t.Run("cannot remove the child owner", func(t *testing.T) {
		fx := newChildAclFixture(t, nil)
		rec := fx.removeNoRotateRecord(t, fx.legalOwnerKeys, fx.ownerKeys.SignKey.GetPublic())
		require.ErrorIs(t, fx.ownerAcl.AddRawRecord(rec), ErrInsufficientPermissions)
	})

	t.Run("unknown identity", func(t *testing.T) {
		fx := newChildAclFixture(t, nil)
		stranger, err := accountdata.NewRandom()
		require.NoError(t, err)
		rec := fx.removeNoRotateRecord(t, fx.legalOwnerKeys, stranger.SignKey.GetPublic())
		require.ErrorIs(t, fx.ownerAcl.AddRawRecord(rec), ErrNoSuchAccount)
	})

	t.Run("empty identities", func(t *testing.T) {
		fx := newChildAclFixture(t, nil)
		rec := fx.removeNoRotateRecord(t, fx.legalOwnerKeys)
		require.ErrorIs(t, fx.ownerAcl.AddRawRecord(rec), ErrIncorrectNumberOfAccounts)
	})

	t.Run("not a child space", func(t *testing.T) {
		a := NewAclExecutor("spaceId")
		require.NoError(t, a.Execute("a.init::a"))
		ownerAcl := a.ActualAccounts()["a"].Acl
		legalOwnerKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		target, err := accountdata.NewRandom()
		require.NoError(t, err)
		protoIdentity, err := target.SignKey.GetPublic().Marshall()
		require.NoError(t, err)
		content := &aclrecordproto.AclContentValue{
			Value: &aclrecordproto.AclContentValue_AccountRemoveNoRotate{
				AccountRemoveNoRotate: &aclrecordproto.AclAccountRemoveNoRotate{Identities: [][]byte{protoIdentity}},
			},
		}
		rec := buildAclRecordSignedBy(t, ownerAcl.Head().Id, legalOwnerKeys, content)
		require.ErrorIs(t, ownerAcl.AddRawRecord(rec), ErrNotChildSpace)
	})
}

func TestKeylessRemove_EditorRotation(t *testing.T) {
	t.Run("writer cannot rotate without the opt-in", func(t *testing.T) {
		fx := newChildAclFixture(t, nil)
		rec := fx.removeNoRotateRecord(t, fx.legalOwnerKeys, fx.writerKeys.SignKey.GetPublic())
		// remove a second member so the writer remains; here remove nobody relevant: add one more writer to remove
		_ = rec
		// add a reader, remove it keylessly, then the writer attempts the rotation
		readerKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		add, err := fx.ownerAcl.RecordBuilder().BuildAccountsAdd(AccountsAddPayload{
			Additions: []AccountAdd{{
				Identity:    readerKeys.SignKey.GetPublic(),
				Permissions: AclPermissionsReader,
				Metadata:    []byte("rm"),
			}},
		})
		require.NoError(t, err)
		addToAll(t, listtest.WrapAclRecord(add), fx.ownerAcl, fx.writerAcl)

		remove := fx.removeNoRotateRecord(t, fx.legalOwnerKeys, readerKeys.SignKey.GetPublic())
		addToAll(t, remove, fx.ownerAcl, fx.writerAcl)
		require.True(t, fx.writerAcl.AclState().HasPendingKeylessRemovals())

		_, err = fx.writerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
		require.ErrorIs(t, err, ErrInsufficientPermissions)
	})

	t.Run("writer rotates with the opt-in and a pending removal", func(t *testing.T) {
		fx := newChildAclFixture(t, &aclrecordproto.AclSpaceOptions{EditorsCanCompleteKeylessRotation: true})
		readerKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		add, err := fx.ownerAcl.RecordBuilder().BuildAccountsAdd(AccountsAddPayload{
			Additions: []AccountAdd{{
				Identity:    readerKeys.SignKey.GetPublic(),
				Permissions: AclPermissionsReader,
				Metadata:    []byte("rm"),
			}},
		})
		require.NoError(t, err)
		addToAll(t, listtest.WrapAclRecord(add), fx.ownerAcl, fx.writerAcl)

		// without a pending removal the writer still cannot rotate
		_, err = fx.writerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
		require.ErrorIs(t, err, ErrInsufficientPermissions)

		remove := fx.removeNoRotateRecord(t, fx.legalOwnerKeys, readerKeys.SignKey.GetPublic())
		addToAll(t, remove, fx.ownerAcl, fx.writerAcl)

		rotation, err := fx.writerAcl.RecordBuilder().BuildReadKeyChange(newTestReadKeyChangePayload())
		require.NoError(t, err)
		addToAll(t, listtest.WrapAclRecord(rotation), fx.ownerAcl, fx.writerAcl)
		require.False(t, fx.ownerAcl.AclState().HasPendingKeylessRemovals())
	})
}
