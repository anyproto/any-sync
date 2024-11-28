package list

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type aclFixture struct {
	ownerKeys   *accountdata.AccountKeys
	accountKeys *accountdata.AccountKeys
	ownerAcl    *aclList
	accountAcl  *aclList
	spaceId     string
}

var mockMetadata = []byte("very important metadata")

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
	require.Equal(t, ownerAcl.AclState().lastRecordId, ownerAcl.Id())
	require.Equal(t, ownerAcl.AclState().lastRecordId, accountAcl.AclState().lastRecordId)
	require.NotEmpty(t, ownerAcl.Id())
	meta, err := ownerAcl.AclState().GetMetadata(ownerKeys.SignKey.GetPublic(), true)
	require.NoError(t, err)
	require.Equal(t, []byte("metadata"), meta)
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
		ownerAcl   = fx.ownerAcl
		ownerState = func() *AclState {
			return ownerAcl.aclState
		}
		accountAcl   = fx.accountAcl
		accountState = func() *AclState {
			return accountAcl.aclState
		}
	)
	// building invite
	inv, err := ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	inviteRec := WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteKey: inv.InviteKey,
		Metadata:  mockMetadata,
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
	err = ownerAcl.ValidateRawRecord(requestAccept, nil)
	require.NoError(t, err)
	requestAcceptRec := WrapAclRecord(requestAccept)
	fx.addRec(t, requestAcceptRec)

	// checking acl state
	for _, acl := range []*aclList{ownerAcl, accountAcl} {
		require.True(t, acl.AclState().Permissions(ownerAcl.AclState().pubKey).IsOwner())
		require.True(t, acl.AclState().Permissions(acl.AclState().pubKey).CanWrite())
		require.Equal(t, 0, len(acl.AclState().pendingRequests))
	}

	permsAtJoinRec, err := ownerState().PermissionsAtRecord(requestJoinRec.Id, accountState().pubKey)
	require.NoError(t, err)
	require.Equal(t, AclPermissionsNone, permsAtJoinRec)
	permsAtRec, err := ownerState().PermissionsAtRecord(requestAcceptRec.Id, accountState().pubKey)
	require.NoError(t, err)
	require.True(t, permsAtRec == perms)
	require.Equal(t, ownerAcl.AclState().lastRecordId, requestAcceptRec.Id)
	require.Equal(t, ownerAcl.AclState().lastRecordId, accountAcl.AclState().lastRecordId)
	require.NotEmpty(t, requestAcceptRec.Id)
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

func TestAclList_PermissionsAtRecord(t *testing.T) {
	t.Run("non-existing record", func(t *testing.T) {
		fx := newFixture(t)
		fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))
		_, err := fx.ownerAcl.aclState.PermissionsAtRecord("some", fx.ownerKeys.SignKey.GetPublic())
		require.Equal(t, ErrNoSuchRecord, err)
	})
}

func TestAclList_InviteRevoke(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerAcl   = fx.ownerAcl
		ownerState = func() *AclState {
			return ownerAcl.aclState
		}
		accountAcl   = fx.accountAcl
		accountState = func() *AclState {
			return accountAcl.aclState
		}
	)
	// building invite
	inv, err := fx.ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	inviteRec := WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building invite revoke
	inviteRevoke, err := fx.ownerAcl.RecordBuilder().BuildInviteRevoke(ownerState().lastRecordId)
	require.NoError(t, err)
	inviteRevokeRec := WrapAclRecord(inviteRevoke)
	fx.addRec(t, inviteRevokeRec)

	// checking acl state
	require.True(t, ownerState().Permissions(ownerState().pubKey).IsOwner())
	require.True(t, ownerState().Permissions(accountState().pubKey).NoPermissions())
	require.Empty(t, ownerState().inviteKeys)
	require.Empty(t, accountState().inviteKeys)
}

func TestAclList_RequestDecline(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerAcl   = fx.ownerAcl
		ownerState = func() *AclState {
			return ownerAcl.aclState
		}
		accountAcl   = fx.accountAcl
		accountState = func() *AclState {
			return accountAcl.aclState
		}
	)
	// building invite
	inv, err := ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	inviteRec := WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteKey: inv.InviteKey,
	})
	require.NoError(t, err)
	requestJoinRec := WrapAclRecord(requestJoin)
	fx.addRec(t, requestJoinRec)

	// building request decline
	requestDecline, err := ownerAcl.RecordBuilder().BuildRequestDecline(ownerState().lastRecordId)
	require.NoError(t, err)
	requestDeclineRec := WrapAclRecord(requestDecline)
	fx.addRec(t, requestDeclineRec)

	// checking acl state
	require.True(t, ownerState().Permissions(ownerState().pubKey).IsOwner())
	require.True(t, ownerState().Permissions(accountState().pubKey).NoPermissions())
	require.Empty(t, ownerState().pendingRequests)
	require.Empty(t, accountState().pendingRequests)
}

func TestAclList_Remove(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerAcl   = fx.ownerAcl
		ownerState = func() *AclState {
			return ownerAcl.aclState
		}
		accountAcl   = fx.accountAcl
		accountState = func() *AclState {
			return accountAcl.aclState
		}
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
	require.True(t, ownerState().Permissions(ownerState().pubKey).IsOwner())
	require.True(t, ownerState().Permissions(accountState().pubKey).NoPermissions())
	require.True(t, ownerState().keys[removeRec.Id].ReadKey.Equals(newReadKey))
	require.True(t, ownerState().keys[removeRec.Id].MetadataPrivKey.Equals(privKey))
	require.True(t, ownerState().keys[removeRec.Id].MetadataPubKey.Equals(pubKey))
	require.NotEmpty(t, ownerState().keys[fx.ownerAcl.Id()])
	require.Equal(t, 0, len(ownerState().pendingRequests))
	require.Equal(t, 0, len(accountState().pendingRequests))
	require.True(t, accountState().Permissions(ownerState().pubKey).IsOwner())
	require.True(t, accountState().Permissions(accountState().pubKey).NoPermissions())
	require.NotEmpty(t, accountState().keys[removeRec.Id])
	require.Nil(t, accountState().keys[removeRec.Id].MetadataPrivKey)
	require.NotNil(t, accountState().keys[removeRec.Id].MetadataPubKey)
	require.Nil(t, accountState().keys[removeRec.Id].ReadKey)
	require.NotEmpty(t, accountState().keys[fx.ownerAcl.Id()])
}

func TestAclList_FixAcceptPanic(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))

	_, err := BuildAclListWithIdentity(fx.accountKeys, fx.ownerAcl.storage, NoOpAcceptorVerifier{})
	require.NoError(t, err)
}

func TestAclList_KeyChangeInvite(t *testing.T) {
	fx := newFixture(t)
	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	readKeyChange, err := fx.ownerAcl.RecordBuilder().BuildReadKeyChange(ReadKeyChangePayload{
		MetadataKey: privKey,
		ReadKey:     newReadKey,
	})
	require.NoError(t, err)
	readKeyRec := WrapAclRecord(readKeyChange)
	fx.addRec(t, readKeyRec)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))
}

func TestAclList_MetadataDecrypt(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))
	meta, err := fx.ownerAcl.AclState().GetMetadata(fx.accountKeys.SignKey.GetPublic(), true)
	require.NoError(t, err)
	require.Equal(t, mockMetadata, meta)
	meta, err = fx.ownerAcl.AclState().GetMetadata(fx.accountKeys.SignKey.GetPublic(), false)
	require.NoError(t, err)
	require.NotEqual(t, mockMetadata, meta)
}

func TestAclList_ReadKeyChange(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerAcl   = fx.ownerAcl
		ownerState = func() *AclState {
			return ownerAcl.aclState
		}
		accountAcl   = fx.accountAcl
		accountState = func() *AclState {
			return accountAcl.aclState
		}
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
	require.True(t, ownerState().Permissions(ownerState().pubKey).IsOwner())
	require.True(t, ownerState().Permissions(accountState().pubKey).CanManageAccounts())
	require.True(t, ownerState().keys[readKeyRec.Id].ReadKey.Equals(newReadKey))
	require.True(t, ownerState().keys[readKeyRec.Id].MetadataPrivKey.Equals(privKey))
	require.True(t, ownerState().keys[readKeyRec.Id].MetadataPubKey.Equals(pubKey))
	require.True(t, accountState().keys[readKeyRec.Id].ReadKey.Equals(newReadKey))
	require.NotEmpty(t, ownerState().keys[fx.ownerAcl.Id()])
	require.NotEmpty(t, accountState().keys[fx.ownerAcl.Id()])
	readKey, err := ownerState().CurrentReadKey()
	require.NoError(t, err)
	require.True(t, newReadKey.Equals(readKey))
	require.Equal(t, 0, len(ownerState().pendingRequests))
	require.Equal(t, 0, len(accountState().pendingRequests))
}

func TestAclList_PermissionChange(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerAcl   = fx.ownerAcl
		ownerState = func() *AclState {
			return ownerAcl.aclState
		}
		accountAcl   = fx.accountAcl
		accountState = func() *AclState {
			return accountAcl.aclState
		}
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
	for _, acl := range []*aclList{fx.ownerAcl, fx.accountAcl} {
		require.True(t, acl.AclState().Permissions(ownerState().pubKey).IsOwner())
		require.True(t, acl.AclState().Permissions(accountState().pubKey).CanWrite())
		require.Equal(t, 0, len(acl.AclState().pendingRequests))
		require.NotEmpty(t, acl.AclState().keys[fx.ownerAcl.Id()])
	}
}

func TestAclList_RequestRemove(t *testing.T) {
	fx := newFixture(t)
	var (
		ownerAcl   = fx.ownerAcl
		ownerState = func() *AclState {
			return ownerAcl.aclState
		}
		accountAcl   = fx.accountAcl
		accountState = func() *AclState {
			return accountAcl.aclState
		}
	)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))

	removeRequest, err := fx.accountAcl.RecordBuilder().BuildRequestRemove()
	require.NoError(t, err)
	removeRequestRec := WrapAclRecord(removeRequest)
	fx.addRec(t, removeRequestRec)

	recs := fx.accountAcl.AclState().RemoveRecords()
	require.Len(t, recs, 1)
	require.True(t, accountState().pubKey.Equals(recs[0].RequestIdentity))

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
	for _, acl := range []*aclList{fx.ownerAcl, fx.accountAcl} {
		require.True(t, acl.AclState().Permissions(ownerState().pubKey).IsOwner())
		require.True(t, acl.AclState().Permissions(accountState().pubKey).NoPermissions())
		require.Equal(t, 0, len(acl.AclState().pendingRequests))
	}
	require.True(t, ownerState().keys[removeRec.Id].ReadKey.Equals(newReadKey))
	require.NotEmpty(t, ownerState().keys[fx.ownerAcl.Id()])
	require.Nil(t, accountState().keys[removeRec.Id].MetadataPrivKey)
	require.NotNil(t, accountState().keys[removeRec.Id].MetadataPubKey)
	require.Nil(t, accountState().keys[removeRec.Id].ReadKey)
	require.NotEmpty(t, accountState().keys[fx.ownerAcl.Id()])
}

func TestAclState_OwnerPubKey(t *testing.T) {
	fx := newFixture(t)
	pubKey, err := fx.ownerAcl.AclState().OwnerPubKey()
	require.NoError(t, err)
	assert.Equal(t, fx.ownerKeys.SignKey.GetPublic().Account(), pubKey.Account())
}
