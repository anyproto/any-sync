package list

import (
	"fmt"
	"testing"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/stretchr/testify/require"
)

func wrapRecord(rawRec *aclrecordproto.RawAclRecord) *aclrecordproto.RawAclRecordWithId {
	payload, err := rawRec.Marshal()
	if err != nil {
		panic(err)
	}
	id, err := cidutil.NewCidFromBytes(payload)
	if err != nil {
		panic(err)
	}
	return &aclrecordproto.RawAclRecordWithId{
		Payload: payload,
		Id:      id,
	}
}

type aclFixture struct {
	ownerKeys   *accountdata.AccountKeys
	accountKeys *accountdata.AccountKeys
	ownerAcl    *aclList
	accountAcl  *aclList
	spaceId     string
}

func newFixture(t *testing.T) *aclFixture {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId := "spaceId"
	ownerAcl, err := NewTestDerivedAcl(spaceId, ownerKeys)
	require.NoError(t, err)
	accountAcl, err := NewTestAclWithRoot(accountKeys, ownerAcl.Root())
	require.NoError(t, err)
	return &aclFixture{
		ownerKeys:   ownerKeys,
		accountKeys: accountKeys,
		ownerAcl:    ownerAcl.(*aclList),
		accountAcl:  accountAcl.(*aclList),
		spaceId:     spaceId,
	}
}

func (fx *aclFixture) addRec(t *testing.T, rec *aclrecordproto.RawAclRecordWithId) {
	err := fx.ownerAcl.AddRawRecord(rec)
	require.NoError(t, err)
	err = fx.accountAcl.AddRawRecord(rec)
	require.NoError(t, err)
}

func (fx *aclFixture) inviteAccount(t *testing.T, perms AclPermissions) {
	var (
		ownerAcl     = fx.ownerAcl
		ownerState   = fx.ownerAcl.aclState
		accountAcl   = fx.accountAcl
		accountState = fx.accountAcl.aclState
	)
	// building invite
	inv, err := ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	inviteRec := wrapRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteRecordId: inviteRec.Id,
		InviteKey:      inv.InviteKey,
	})
	require.NoError(t, err)
	requestJoinRec := wrapRecord(requestJoin)
	fx.addRec(t, requestJoinRec)

	// building request accept
	requestAccept, err := ownerAcl.RecordBuilder().BuildRequestAccept(RequestAcceptPayload{
		RequestRecordId: requestJoinRec.Id,
		Permissions:     perms,
	})
	require.NoError(t, err)
	requestAcceptRec := wrapRecord(requestAccept)
	fx.addRec(t, requestAcceptRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).CanWrite())
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
	require.True(t, accountState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, accountState.Permissions(accountState.pubKey).CanWrite())
}

func TestAclList_BuildRoot(t *testing.T) {
	randomKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	randomAcl, err := NewTestDerivedAcl("spaceId", randomKeys)
	require.NoError(t, err)
	fmt.Println(randomAcl.Id())
}

func TestAclList_InvitePipeline(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))
}

func TestAclList_Remove(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerState   = fx.ownerAcl.aclState
		accountState = fx.accountAcl.aclState
	)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))

	newReadKey := crypto.NewAES()
	remove, err := fx.ownerAcl.RecordBuilder().BuildAccountRemove(AccountRemovePayload{
		Identities: []crypto.PubKey{fx.accountKeys.SignKey.GetPublic()},
		ReadKey:    newReadKey,
	})
	require.NoError(t, err)
	removeRec := wrapRecord(remove)
	fx.addRec(t, removeRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).NoPermissions())
	require.True(t, ownerState.userReadKeys[removeRec.Id].Equals(newReadKey))
	require.NotNil(t, ownerState.userReadKeys[fx.ownerAcl.Id()])
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
	require.True(t, accountState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, accountState.Permissions(accountState.pubKey).NoPermissions())
	require.Nil(t, accountState.userReadKeys[removeRec.Id])
	require.NotNil(t, accountState.userReadKeys[fx.ownerAcl.Id()])
}
