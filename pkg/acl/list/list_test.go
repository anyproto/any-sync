package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/acllistbuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAclList_ACLState_UserInviteAndJoin(t *testing.T) {
	st, err := acllistbuilder.NewListStorageWithTestName("userjoinexample.yml")
	require.NoError(t, err, "building storage should not result in error")

	keychain := st.(*acllistbuilder.ACLListStorageBuilder).GetKeychain()

	aclList, err := BuildACLList(signingkey.NewEDPubKeyDecoder(), st)
	require.NoError(t, err, "building acl list should be without error")

	idA := keychain.GetIdentity("A")
	idB := keychain.GetIdentity("B")
	idC := keychain.GetIdentity("C")

	// checking final state
	assert.Equal(t, aclList.ACLState().GetUserStates()[idA].Permissions, aclpb.ACLChange_Admin)
	assert.Equal(t, aclList.ACLState().GetUserStates()[idB].Permissions, aclpb.ACLChange_Writer)
	assert.Equal(t, aclList.ACLState().GetUserStates()[idC].Permissions, aclpb.ACLChange_Reader)
	assert.Equal(t, aclList.ACLState().CurrentReadKeyHash(), aclList.Head().Content.CurrentReadKeyHash)
	var records []*Record
	aclList.Iterate(func(record *Record) (IsContinue bool) {
		records = append(records, record)
		return true
	})

	// checking permissions at specific records
	assert.Equal(t, 3, len(records))
	_, err = aclList.ACLState().PermissionsAtRecord(records[1].Id, idB)
	assert.Error(t, err, "B should have no permissions at record 1")
	perm, err := aclList.ACLState().PermissionsAtRecord(records[2].Id, idB)
	assert.NoError(t, err, "should have no error with permissions of B in the record 2")
	assert.Equal(t, perm, UserPermissionPair{
		Identity:   idB,
		Permission: aclpb.ACLChange_Writer,
	})
}
