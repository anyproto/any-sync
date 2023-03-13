package list

import (
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	acllistbuilder "github.com/anytypeio/any-sync/commonspace/object/acl/testutils/acllistbuilder"
	"github.com/anytypeio/any-sync/commonspace/object/keychain"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAclRecordBuilder_BuildUserJoin(t *testing.T) {
	st, err := acllistbuilder.NewListStorageWithTestName("userjoinexample.yml")
	require.NoError(t, err, "building storage should not result in error")

	testKeychain := st.(*acllistbuilder.AclListStorageBuilder).GetKeychain()
	identity := testKeychain.GeneratedIdentities["D"]
	signPrivKey := testKeychain.SigningKeysByYAMLName["D"]
	encPrivKey := testKeychain.EncryptionKeysByYAMLName["D"]
	acc := &accountdata.AccountData{
		Identity: []byte(identity),
		SignKey:  signPrivKey,
		EncKey:   encPrivKey,
	}

	aclList, err := BuildAclListWithIdentity(acc, st)
	require.NoError(t, err, "building acl list should be without error")
	recordBuilder := newAclRecordBuilder(aclList.Id(), keychain.NewKeychain())
	rk, err := testKeychain.GetKey("key.Read.EncKey").(*acllistbuilder.SymKey).Key.Raw()
	require.NoError(t, err)
	privKey, err := testKeychain.GetKey("key.Sign.Onetime1").(signingkey.PrivKey).Raw()
	require.NoError(t, err)

	userJoin, err := recordBuilder.BuildUserJoin(privKey, rk, aclList.AclState())
	require.NoError(t, err)
	marshalledJoin, err := userJoin.Marshal()
	require.NoError(t, err)
	id, err := cidutil.NewCidFromBytes(marshalledJoin)
	require.NoError(t, err)
	rawRec := &aclrecordproto.RawAclRecordWithId{
		Payload: marshalledJoin,
		Id:      id,
	}
	res, err := aclList.AddRawRecord(rawRec)
	require.True(t, res)
	require.NoError(t, err)
	require.Equal(t, aclrecordproto.AclUserPermissions_Writer, aclList.AclState().UserStates()[identity].Permissions)
}
