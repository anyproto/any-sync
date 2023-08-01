package list

import (
	"fmt"
	"testing"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/stretchr/testify/require"
)

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

func (fx *aclFixture) addRec(t *testing.T, rec *consensusproto.RawRecordWithId) {
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
	inviteRec := WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteRecordId: inviteRec.Id,
		InviteKey:      inv.InviteKey,
	})
	require.NoError(t, err)
	requestJoinRec := WrapAclRecord(requestJoin)
	fx.addRec(t, requestJoinRec)

	// building request accept
	requestAccept, err := ownerAcl.RecordBuilder().BuildRequestAccept(RequestAcceptPayload{
		RequestRecordId: requestJoinRec.Id,
		Permissions:     perms,
	})
	require.NoError(t, err)
	// validate
	err = ownerAcl.ValidateRawRecord(requestAccept)
	require.NoError(t, err)
	requestAcceptRec := WrapAclRecord(requestAccept)
	fx.addRec(t, requestAcceptRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).CanWrite())
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
	require.True(t, accountState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, accountState.Permissions(accountState.pubKey).CanWrite())

	_, err = ownerState.StateAtRecord(requestJoinRec.Id, accountState.pubKey)
	require.Equal(t, ErrNoSuchAccount, err)
	stateAtRec, err := ownerState.StateAtRecord(requestAcceptRec.Id, accountState.pubKey)
	require.NoError(t, err)
	require.True(t, stateAtRec.Permissions == perms)
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

func TestAclList_InviteRevoke(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerState   = fx.ownerAcl.aclState
		accountState = fx.accountAcl.aclState
	)
	// building invite
	inv, err := fx.ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	inviteRec := WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building invite revoke
	inviteRevoke, err := fx.ownerAcl.RecordBuilder().BuildInviteRevoke(ownerState.lastRecordId)
	require.NoError(t, err)
	inviteRevokeRec := WrapAclRecord(inviteRevoke)
	fx.addRec(t, inviteRevokeRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).NoPermissions())
	require.Empty(t, ownerState.inviteKeys)
	require.Empty(t, accountState.inviteKeys)
}

func TestAclList_RequestDecline(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerAcl     = fx.ownerAcl
		ownerState   = fx.ownerAcl.aclState
		accountAcl   = fx.accountAcl
		accountState = fx.accountAcl.aclState
	)
	// building invite
	inv, err := ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	inviteRec := WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteRecordId: inviteRec.Id,
		InviteKey:      inv.InviteKey,
	})
	require.NoError(t, err)
	requestJoinRec := WrapAclRecord(requestJoin)
	fx.addRec(t, requestJoinRec)

	// building request decline
	requestDecline, err := ownerAcl.RecordBuilder().BuildRequestDecline(ownerState.lastRecordId)
	require.NoError(t, err)
	requestDeclineRec := WrapAclRecord(requestDecline)
	fx.addRec(t, requestDeclineRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).NoPermissions())
	require.Empty(t, ownerState.pendingRequests)
	require.Empty(t, accountState.pendingRequests)
}

func TestAclList_Remove(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerState   = fx.ownerAcl.aclState
		accountState = fx.accountAcl.aclState
	)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))

	newReadKey := crypto.NewAES()
	privKey, pubKey, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	remove, err := fx.ownerAcl.RecordBuilder().BuildAccountRemove(AccountRemovePayload{
		Identities: []crypto.PubKey{fx.accountKeys.SignKey.GetPublic()},
		Change: ReadKeyChangePayload{
			MetadataKey: privKey,
			ReadKey:     newReadKey,
		},
	})
	require.NoError(t, err)
	removeRec := WrapAclRecord(remove)
	fx.addRec(t, removeRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).NoPermissions())
	require.True(t, ownerState.keys[removeRec.Id].ReadKey.Equals(newReadKey))
	require.True(t, ownerState.keys[removeRec.Id].MetadataPrivKey.Equals(privKey))
	require.True(t, ownerState.keys[removeRec.Id].MetadataPubKey.Equals(pubKey))
	require.NotEmpty(t, ownerState.keys[fx.ownerAcl.Id()])
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
	require.True(t, accountState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, accountState.Permissions(accountState.pubKey).NoPermissions())
	require.Empty(t, accountState.keys[removeRec.Id])
	require.NotEmpty(t, accountState.keys[fx.ownerAcl.Id()])
}

func TestAclList_ReadKeyChange(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerState   = fx.ownerAcl.aclState
		accountState = fx.accountAcl.aclState
	)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Admin))

	newReadKey := crypto.NewAES()
	privKey, pubKey, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	readKeyChange, err := fx.ownerAcl.RecordBuilder().BuildReadKeyChange(ReadKeyChangePayload{
		MetadataKey: privKey,
		ReadKey:     newReadKey,
	})
	require.NoError(t, err)
	readKeyRec := WrapAclRecord(readKeyChange)
	fx.addRec(t, readKeyRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).CanManageAccounts())
	require.True(t, ownerState.keys[readKeyRec.Id].ReadKey.Equals(newReadKey))
	require.True(t, ownerState.keys[readKeyRec.Id].MetadataPrivKey.Equals(privKey))
	require.True(t, ownerState.keys[readKeyRec.Id].MetadataPubKey.Equals(pubKey))
	require.True(t, accountState.keys[readKeyRec.Id].ReadKey.Equals(newReadKey))
	require.NotEmpty(t, ownerState.keys[fx.ownerAcl.Id()])
	require.NotEmpty(t, accountState.keys[fx.ownerAcl.Id()])
	readKey, err := ownerState.CurrentReadKey()
	require.NoError(t, err)
	require.True(t, newReadKey.Equals(readKey))
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
}

func TestAclList_PermissionChange(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerState   = fx.ownerAcl.aclState
		accountState = fx.accountAcl.aclState
	)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Admin))

	permissionChange, err := fx.ownerAcl.RecordBuilder().BuildPermissionChange(PermissionChangePayload{
		Identity:    fx.accountKeys.SignKey.GetPublic(),
		Permissions: AclPermissions(aclrecordproto.AclUserPermissions_Writer),
	})
	require.NoError(t, err)
	permissionChangeRec := WrapAclRecord(permissionChange)
	fx.addRec(t, permissionChangeRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey) == AclPermissions(aclrecordproto.AclUserPermissions_Writer))
	require.True(t, accountState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, accountState.Permissions(accountState.pubKey) == AclPermissions(aclrecordproto.AclUserPermissions_Writer))
	require.NotEmpty(t, ownerState.keys[fx.ownerAcl.Id()])
	require.NotEmpty(t, accountState.keys[fx.ownerAcl.Id()])
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
}

func TestAclList_RequestRemove(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerState   = fx.ownerAcl.aclState
		accountState = fx.accountAcl.aclState
	)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))

	removeRequest, err := fx.accountAcl.RecordBuilder().BuildRequestRemove()
	require.NoError(t, err)
	removeRequestRec := WrapAclRecord(removeRequest)
	fx.addRec(t, removeRequestRec)

	recs := fx.accountAcl.AclState().RemoveRecords()
	require.Len(t, recs, 1)
	require.True(t, accountState.pubKey.Equals(recs[0].RequestIdentity))

	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	remove, err := fx.ownerAcl.RecordBuilder().BuildAccountRemove(AccountRemovePayload{
		Identities: []crypto.PubKey{recs[0].RequestIdentity},
		Change: ReadKeyChangePayload{
			MetadataKey: privKey,
			ReadKey:     newReadKey,
		},
	})
	require.NoError(t, err)
	removeRec := WrapAclRecord(remove)
	fx.addRec(t, removeRec)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).NoPermissions())
	require.True(t, ownerState.keys[removeRec.Id].ReadKey.Equals(newReadKey))
	require.NotEmpty(t, ownerState.keys[fx.ownerAcl.Id()])
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
	require.True(t, accountState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, accountState.Permissions(accountState.pubKey).NoPermissions())
	require.Empty(t, accountState.keys[removeRec.Id])
	require.NotEmpty(t, accountState.keys[fx.ownerAcl.Id()])
}
