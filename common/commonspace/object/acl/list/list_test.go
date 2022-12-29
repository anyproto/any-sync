package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/testutils/acllistbuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAclList_AclState_UserInviteAndJoin(t *testing.T) {
	st, err := acllistbuilder.NewListStorageWithTestName("userjoinexample.yml")
	require.NoError(t, err, "building storage should not result in error")

	keychain := st.(*acllistbuilder.AclListStorageBuilder).GetKeychain()

	aclList, err := BuildAclList(st)
	require.NoError(t, err, "building acl list should be without error")

	idA := keychain.GetIdentity("A")
	idB := keychain.GetIdentity("B")
	idC := keychain.GetIdentity("C")

	// checking final state
	assert.Equal(t, aclrecordproto.AclUserPermissions_Admin, aclList.AclState().UserStates()[idA].Permissions)
	assert.Equal(t, aclrecordproto.AclUserPermissions_Writer, aclList.AclState().UserStates()[idB].Permissions)
	assert.Equal(t, aclrecordproto.AclUserPermissions_Reader, aclList.AclState().UserStates()[idC].Permissions)
	assert.Equal(t, aclList.Head().CurrentReadKeyHash, aclList.AclState().CurrentReadKeyHash())

	var records []*AclRecord
	aclList.Iterate(func(record *AclRecord) (IsContinue bool) {
		records = append(records, record)
		return true
	})

	// checking permissions at specific records
	assert.Equal(t, 3, len(records))

	_, err = aclList.AclState().PermissionsAtRecord(records[1].Id, idB)
	assert.Error(t, err, "B should have no permissions at record 1")

	perm, err := aclList.AclState().PermissionsAtRecord(records[2].Id, idB)
	assert.NoError(t, err, "should have no error with permissions of B in the record 2")
	assert.Equal(t, UserPermissionPair{
		Identity:   idB,
		Permission: aclrecordproto.AclUserPermissions_Writer,
	}, perm)
}

func TestAclList_AclState_UserJoinAndRemove(t *testing.T) {
	st, err := acllistbuilder.NewListStorageWithTestName("userremoveexample.yml")
	require.NoError(t, err, "building storage should not result in error")

	keychain := st.(*acllistbuilder.AclListStorageBuilder).GetKeychain()

	aclList, err := BuildAclList(st)
	require.NoError(t, err, "building acl list should be without error")

	idA := keychain.GetIdentity("A")
	idB := keychain.GetIdentity("B")
	idC := keychain.GetIdentity("C")

	// checking final state
	assert.Equal(t, aclrecordproto.AclUserPermissions_Admin, aclList.AclState().UserStates()[idA].Permissions)
	assert.Equal(t, aclrecordproto.AclUserPermissions_Reader, aclList.AclState().UserStates()[idC].Permissions)
	assert.Equal(t, aclList.Head().CurrentReadKeyHash, aclList.AclState().CurrentReadKeyHash())

	_, exists := aclList.AclState().UserStates()[idB]
	assert.Equal(t, false, exists)

	var records []*AclRecord
	aclList.Iterate(func(record *AclRecord) (IsContinue bool) {
		records = append(records, record)
		return true
	})

	// checking permissions at specific records
	assert.Equal(t, 4, len(records))

	assert.NotEqual(t, records[2].CurrentReadKeyHash, aclList.AclState().CurrentReadKeyHash())

	perm, err := aclList.AclState().PermissionsAtRecord(records[2].Id, idB)
	assert.NoError(t, err, "should have no error with permissions of B in the record 2")
	assert.Equal(t, UserPermissionPair{
		Identity:   idB,
		Permission: aclrecordproto.AclUserPermissions_Writer,
	}, perm)

	_, err = aclList.AclState().PermissionsAtRecord(records[3].Id, idB)
	assert.Error(t, err, "B should have no permissions at record 3, because user should be removed")
}
