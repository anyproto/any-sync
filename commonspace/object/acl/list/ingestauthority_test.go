package list

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

// A list that validates in full what it ingests has no coordinator to ask, so in any space nobody has
// the authority of a parent space there.
func TestIngest_NoParentAuthorityUnderFullValidation(t *testing.T) {
	newSpace := func(t *testing.T) (fx *aclFixture, owner, writer crypto.PubKey, head string) {
		fx = newFixture(t)
		fx.inviteAccount(t, AclPermissionsWriter)
		require.True(t, fx.ownerAcl.verifier.ShouldValidate())
		return fx, fx.ownerKeys.SignKey.GetPublic(), fx.accountKeys.SignKey.GetPublic(), fx.ownerAcl.Head().Id
	}
	requireUnchanged := func(t *testing.T, fx *aclFixture, owner, writer crypto.PubKey, head string) {
		st := fx.ownerAcl.AclState()
		require.Equal(t, head, st.LastRecordId())
		require.True(t, st.Permissions(owner).IsOwner())
		require.Equal(t, AclPermissionsWriter, st.Permissions(writer))
	}

	t.Run("a Writer does not make itself the owner", func(t *testing.T) {
		fx, owner, writer, head := newSpace(t)
		rec, err := fx.accountAcl.RecordBuilder().BuildOwnershipChangeByParent(OwnershipChangePayload{
			NewOwner:            writer,
			OldOwnerPermissions: AclPermissionsReader,
		})
		require.NoError(t, err)
		wrapped := listtest.WrapAclRecord(rec)
		require.ErrorIs(t, fx.ownerAcl.AddRawRecord(wrapped), ErrInsufficientPermissions)
		require.ErrorIs(t, fx.ownerAcl.AddRawRecords([]*consensusproto.RawRecordWithId{wrapped}), ErrInsufficientPermissions)
		requireUnchanged(t, fx, owner, writer, head)
	})

	t.Run("an ownership change signed by a Writer is refused", func(t *testing.T) {
		fx, owner, writer, head := newSpace(t)
		newOwner, err := writer.Marshall()
		require.NoError(t, err)
		data, err := (&aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{{
			Value: &aclrecordproto.AclContentValue_OwnershipChange{OwnershipChange: &aclrecordproto.AclOwnershipChange{
				NewOwnerIdentity:    newOwner,
				OldOwnerPermissions: aclrecordproto.AclUserPermissions_Reader,
			}},
		}}}).MarshalVT()
		require.NoError(t, err)
		_, wrapped := signAclRecord(t, fx.accountKeys, head, data)
		require.ErrorIs(t, fx.ownerAcl.AddRawRecord(wrapped), ErrInsufficientPermissions)
		requireUnchanged(t, fx, owner, writer, head)
	})
}
