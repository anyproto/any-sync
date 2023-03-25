package list

import (
	"testing"
)

func TestAclRecordBuilder_BuildUserJoin(t *testing.T) {
	//st, err := acllistbuilder2.NewListStorageWithTestName("userjoinexample.yml")
	//require.NoError(t, err, "building storage should not result in error")
	//
	//testKeychain := st.(*acllistbuilder2.AclListStorageBuilder).GetKeychain()
	//identity := testKeychain.GeneratedIdentities["D"]
	//signPrivKey := testKeychain.SigningKeysByYAMLName["D"]
	//encPrivKey := testKeychain.EncryptionKeysByYAMLName["D"]
	//acc := &accountdata.AccountKeys{
	//	Identity: []byte(identity),
	//	PrivKey:  signPrivKey,
	//	EncKey:   encPrivKey,
	//}
	//
	//aclList, err := BuildAclListWithIdentity(acc, st)
	//require.NoError(t, err, "building acl list should be without error")
	//recordBuilder := newAclRecordBuilder(aclList.Id(), keychain.NewKeychain())
	//rk, err := testKeychain.GetKey("key.Read.EncKey").(*acllistbuilder2.SymKey).Key.Raw()
	//require.NoError(t, err)
	//privKey, err := testKeychain.GetKey("key.Sign.Onetime1").(signingkey.PrivKey).Raw()
	//require.NoError(t, err)
	//
	//userJoin, err := recordBuilder.BuildUserJoin(privKey, rk, aclList.AclState())
	//require.NoError(t, err)
	//marshalledJoin, err := userJoin.Marshal()
	//require.NoError(t, err)
	//id, err := cidutil.NewCidFromBytes(marshalledJoin)
	//require.NoError(t, err)
	//rawRec := &aclrecordproto.RawAclRecordWithId{
	//	Payload: marshalledJoin,
	//	Id:      id,
	//}
	//res, err := aclList.AddRawRecord(rawRec)
	//require.True(t, res)
	//require.NoError(t, err)
	//require.Equal(t, aclrecordproto.AclUserPermissions_Writer, aclList.AclState().UserStates()[identity].Permissions)
}
