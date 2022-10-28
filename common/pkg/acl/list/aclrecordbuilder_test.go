package list

import (
	"context"
	account "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/testutils/acllistbuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAclRecordBuilder_BuildUserJoin(t *testing.T) {
	st, err := acllistbuilder.NewListStorageWithTestName("userjoinexample.yml")
	require.NoError(t, err, "building storage should not result in error")

	keychain := st.(*acllistbuilder.ACLListStorageBuilder).GetKeychain()
	identity := keychain.GeneratedIdentities["D"]
	signPrivKey := keychain.SigningKeysByYAMLName["D"]
	encPrivKey := keychain.EncryptionKeysByYAMLName["D"]
	acc := &account.AccountData{
		Identity: []byte(identity),
		SignKey:  signPrivKey,
		EncKey:   encPrivKey,
	}

	aclList, err := BuildACLListWithIdentity(acc, st)
	require.NoError(t, err, "building acl list should be without error")
	recordBuilder := newACLRecordBuilder(aclList.ID(), common.NewKeychain())
	rk, err := keychain.GetKey("key.Read.EncKey").(*acllistbuilder.SymKey).Key.Raw()
	require.NoError(t, err)
	privKey, err := keychain.GetKey("key.Sign.Onetime1").(signingkey.PrivKey).Raw()
	require.NoError(t, err)

	userJoin, err := recordBuilder.BuildUserJoin(privKey, rk, aclList.ACLState())
	require.NoError(t, err)
	marshalledJoin, err := userJoin.Marshal()
	require.NoError(t, err)
	id, err := cid.NewCIDFromBytes(marshalledJoin)
	require.NoError(t, err)
	rawRec := &aclrecordproto.RawACLRecordWithId{
		Payload: marshalledJoin,
		Id:      id,
	}
	err = aclList.AddRawRecords(context.Background(), []*aclrecordproto.RawACLRecordWithId{rawRec})
	require.NoError(t, err)
	require.Equal(t, aclrecordproto.ACLUserPermissions_Writer, aclList.ACLState().UserStates()[identity].Permissions)
}
