package list

import (
	"fmt"
	"testing"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/cidutil"
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

func TestAclList_BuildRoot(t *testing.T) {
	randomKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	randomAcl, err := NewTestDerivedAcl("spaceId", randomKeys)
	require.NoError(t, err)
	fmt.Println(randomAcl.Id())
}

func TestAclList_InvitePipeline(t *testing.T) {
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
	inviteRec := wrapRecord(inv.InviteRec)
	err = ownerAcl.AddRawRecord(inviteRec)
	require.NoError(t, err)
	err = accountAcl.AddRawRecord(inviteRec)
	require.NoError(t, err)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteRecordId: inviteRec.Id,
		InviteKey:      inv.InviteKey,
	})
	require.NoError(t, err)
	requestJoinRec := wrapRecord(requestJoin)
	err = ownerAcl.AddRawRecord(requestJoinRec)
	require.NoError(t, err)
	err = accountAcl.AddRawRecord(requestJoinRec)
	require.NoError(t, err)

	// building request accept
	requestAccept, err := ownerAcl.RecordBuilder().BuildRequestAccept(RequestAcceptPayload{
		RequestRecordId: requestJoinRec.Id,
		Permissions:     AclPermissions(aclrecordproto.AclUserPermissions_Writer),
	})
	require.NoError(t, err)
	requestAcceptRec := wrapRecord(requestAccept)
	err = ownerAcl.AddRawRecord(requestAcceptRec)
	require.NoError(t, err)
	err = accountAcl.AddRawRecord(requestAcceptRec)
	require.NoError(t, err)

	// checking acl state
	require.True(t, ownerState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, ownerState.Permissions(accountState.pubKey).CanWrite())
	require.Equal(t, 0, len(ownerState.pendingRequests))
	require.Equal(t, 0, len(accountState.pendingRequests))
	require.True(t, accountState.Permissions(ownerState.pubKey).IsOwner())
	require.True(t, accountState.Permissions(accountState.pubKey).CanWrite())
}
