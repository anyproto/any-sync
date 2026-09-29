package list

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func signTestAclRecord(t *testing.T, keys *accountdata.AccountKeys, prevId string, content ...*aclrecordproto.AclContentValue) (*consensusproto.RawRecord, *consensusproto.RawRecordWithId) {
	data, err := (&aclrecordproto.AclData{AclContent: content}).MarshalVT()
	require.NoError(t, err)
	identity, err := keys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	payload, err := (&consensusproto.Record{
		PrevId:    prevId,
		Identity:  identity,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}).MarshalVT()
	require.NoError(t, err)
	signature, err := keys.SignKey.Sign(payload)
	require.NoError(t, err)
	raw := &consensusproto.RawRecord{Payload: payload, Signature: signature}
	return raw, listtest.WrapAclRecord(raw)
}

// singleContent returns the only content value of a record the builder made.
func singleContent(t *testing.T, raw *consensusproto.RawRecord) *aclrecordproto.AclContentValue {
	rec := &consensusproto.Record{}
	require.NoError(t, rec.UnmarshalVT(raw.Payload))
	data := &aclrecordproto.AclData{}
	require.NoError(t, data.UnmarshalVT(rec.Data))
	require.Len(t, data.AclContent, 1)
	return data.AclContent[0]
}

// requestJoinContent returns the content of a valid join request by the fixture's account, made through a
// request-to-join invite the owner has just added.
func requestJoinContent(t *testing.T, fx *aclFixture) *aclrecordproto.AclContentValue {
	inv, err := fx.ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	fx.addRec(t, listtest.WrapAclRecord(inv.InviteRec))
	joinRaw, err := fx.accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{InviteKey: inv.InviteKey, Metadata: mockMetadata})
	require.NoError(t, err)
	return singleContent(t, joinRaw)
}

func requestToJoinInviteContent(t *testing.T) *aclrecordproto.AclContentValue {
	_, pub, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	key, err := pub.Marshall()
	require.NoError(t, err)
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_Invite{Invite: &aclrecordproto.AclAccountInvite{
		InviteKey:  key,
		InviteType: aclrecordproto.AclInviteType_RequestToJoin,
	}}}
}

func requestCancelContent(recordId string) *aclrecordproto.AclContentValue {
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestCancel{
		RequestCancel: &aclrecordproto.AclAccountRequestCancel{RecordId: recordId},
	}}
}

func inviteRevokeContent(inviteRecordId string) *aclrecordproto.AclContentValue {
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteRevoke{
		InviteRevoke: &aclrecordproto.AclAccountInviteRevoke{InviteRecordId: inviteRecordId},
	}}
}

// emptyStorage returns a fresh storage holding only the root of the fixture's acl.
func emptyStorage(t *testing.T, fx *aclFixture) Storage {
	ctx := context.Background()
	store := createStore(ctx, t)
	headStorage, err := headstorage.New(ctx, store)
	require.NoError(t, err)
	storage, err := CreateStorage(ctx, fx.ownerAcl.Root(), headStorage, store)
	require.NoError(t, err)
	return storage
}

// What a record creates is keyed by a provisional id before the network accepts it, so a later value in
// the record cannot refer to it: such a reference would resolve at admission but not on replay under the
// real id. Admission refuses it, whoever can author the pair.
func TestAclList_ValidateRawRecordRejectsSelfReference(t *testing.T) {
	for name, tc := range map[string]struct {
		record func(t *testing.T, fx *aclFixture) *consensusproto.RawRecord
		err    error
	}{
		"join request cancelled by the joiner": {
			record: func(t *testing.T, fx *aclFixture) *consensusproto.RawRecord {
				join := requestJoinContent(t, fx)
				raw, _ := signTestAclRecord(t, fx.accountKeys, fx.ownerAcl.AclState().LastRecordId(), join, requestCancelContent(""))
				return raw
			},
			err: ErrNoSuchRequest,
		},
		"removal request cancelled by the member": {
			record: func(t *testing.T, fx *aclFixture) *consensusproto.RawRecord {
				fx.inviteAccount(t, AclPermissionsWriter)
				remove := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRequestRemove{
					AccountRequestRemove: &aclrecordproto.AclAccountRequestRemove{},
				}}
				raw, _ := signTestAclRecord(t, fx.accountKeys, fx.ownerAcl.AclState().LastRecordId(), remove, requestCancelContent(""))
				return raw
			},
			err: ErrNoSuchRequest,
		},
		"invite revoked by the owner": {
			record: func(t *testing.T, fx *aclFixture) *consensusproto.RawRecord {
				raw, _ := signTestAclRecord(t, fx.ownerKeys, fx.ownerAcl.AclState().LastRecordId(), requestToJoinInviteContent(t), inviteRevokeContent(""))
				return raw
			},
			err: ErrNoSuchInvite,
		},
		"invite changed by the owner": {
			record: func(t *testing.T, fx *aclFixture) *consensusproto.RawRecord {
				inv, err := fx.ownerAcl.RecordBuilder().BuildInviteAnyone(AclPermissionsReader)
				require.NoError(t, err)
				change := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteChange{
					InviteChange: &aclrecordproto.AclAccountInviteChange{Permissions: aclrecordproto.AclUserPermissions_Writer},
				}}
				raw, _ := signTestAclRecord(t, fx.ownerKeys, fx.ownerAcl.AclState().LastRecordId(), singleContent(t, inv.InviteRec), change)
				return raw
			},
			err: ErrNoSuchInvite,
		},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newFixture(t)
			raw := tc.record(t, fx)
			head := fx.ownerAcl.AclState().LastRecordId()
			require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(raw, nil), tc.err)
			require.Equal(t, head, fx.ownerAcl.AclState().LastRecordId())
		})
	}
}

// A log can still hold a record whose later value refers to what an earlier one created, admitted by a
// network that did not refuse it. Replay keeps applying it as network clients always have, since later
// records were admitted against that state, and never panics.
func TestAclList_ReplayStoredSelfReference(t *testing.T) {
	t.Run("an invite revoked in its own record stays live for a later join", func(t *testing.T) {
		fx := newFixture(t)
		inv, err := fx.ownerAcl.RecordBuilder().BuildInviteAnyone(AclPermissionsWriter)
		require.NoError(t, err)
		owner, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
		require.NoError(t, err)
		_, withInvite := signTestAclRecord(t, fx.ownerKeys, owner.AclState().LastRecordId(), singleContent(t, inv.InviteRec), inviteRevokeContent(""))
		require.NoError(t, owner.AddRawRecord(withInvite))
		require.Len(t, owner.AclState().Invites(), 1)

		// the joiner's client sees the same live invite and joins through it
		joinerList, err := BuildAclListWithIdentity(fx.accountKeys, emptyStorage(t, fx), noValidateVerifier{})
		require.NoError(t, err)
		require.NoError(t, joinerList.AddRawRecord(withInvite))
		joinRaw, err := joinerList.RecordBuilder().BuildInviteJoinWithoutApprove(InviteJoinPayload{
			InviteKey:   inv.InviteKey,
			Permissions: AclPermissionsWriter,
			Metadata:    mockMetadata,
		})
		require.NoError(t, err)
		join := listtest.WrapAclRecord(joinRaw)

		joiner := fx.accountKeys.SignKey.GetPublic()
		require.NoError(t, owner.AddRawRecord(join))
		require.Equal(t, AclPermissionsWriter, owner.AclState().Permissions(joiner))

		rebuilt, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
		require.NoError(t, err)
		require.Equal(t, join.Id, rebuilt.AclState().LastRecordId())
		require.Equal(t, AclPermissionsWriter, rebuilt.AclState().Permissions(joiner))
		require.Len(t, rebuilt.AclState().Invites(), 1)
	})

	t.Run("a join request cancelled in its own record stays pending", func(t *testing.T) {
		fx := newFixture(t)
		join := requestJoinContent(t, fx)
		l, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
		require.NoError(t, err)
		_, withId := signTestAclRecord(t, fx.accountKeys, l.AclState().LastRecordId(), join, requestCancelContent(""))

		joinerKey := mapKeyFromPubKey(fx.accountKeys.SignKey.GetPublic())
		check := func(st *AclState) {
			require.Equal(t, StatusJoining, st.accountStates[joinerKey].Status)
			require.Equal(t, withId.Id, st.pendingRequests[joinerKey])
			require.Contains(t, st.requestRecords, withId.Id)
			require.NotContains(t, st.requestRecords, "")
		}
		require.NotPanics(t, func() { err = l.AddRawRecord(withId) })
		require.NoError(t, err)
		check(l.AclState())

		rebuilt, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
		require.NoError(t, err)
		check(rebuilt.AclState())
	})
}

// A network client does not validate content, so a record in the log whose request or invite reference
// does not resolve must apply that value as a no-op: no panic, and no half-built entry.
func TestAclList_ReplayUnresolvedReference(t *testing.T) {
	stranger, err := accountdata.NewRandom()
	require.NoError(t, err)
	strangerIdentity, err := stranger.SignKey.GetPublic().Marshall()
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		author  func(fx *aclFixture) *accountdata.AccountKeys
		content *aclrecordproto.AclContentValue
	}{
		"request cancel": {
			author:  func(*aclFixture) *accountdata.AccountKeys { return stranger },
			content: requestCancelContent("missing"),
		},
		"request decline": {
			author: func(fx *aclFixture) *accountdata.AccountKeys { return fx.ownerKeys },
			content: &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestDecline{
				RequestDecline: &aclrecordproto.AclAccountRequestDecline{RequestRecordId: "missing"},
			}},
		},
		"request accept": {
			author: func(fx *aclFixture) *accountdata.AccountKeys { return fx.ownerKeys },
			content: &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestAccept{
				RequestAccept: &aclrecordproto.AclAccountRequestAccept{
					Identity:        strangerIdentity,
					RequestRecordId: "missing",
					Permissions:     aclrecordproto.AclUserPermissions_Writer,
				},
			}},
		},
		"invite change": {
			author: func(fx *aclFixture) *accountdata.AccountKeys { return fx.ownerKeys },
			content: &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteChange{
				InviteChange: &aclrecordproto.AclAccountInviteChange{
					InviteRecordId: "missing",
					Permissions:    aclrecordproto.AclUserPermissions_Writer,
				},
			}},
		},
		"invite join": {
			author: func(*aclFixture) *accountdata.AccountKeys { return stranger },
			content: &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteJoin{
				InviteJoin: &aclrecordproto.AclAccountInviteJoin{
					Identity:       strangerIdentity,
					InviteRecordId: "missing",
					Permissions:    aclrecordproto.AclUserPermissions_Writer,
				},
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newFixture(t)
			l, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
			require.NoError(t, err)
			accounts := len(l.AclState().accountStates)
			check := func(st *AclState) {
				require.Empty(t, st.invites)
				require.Empty(t, st.requestRecords)
				require.Empty(t, st.pendingRequests)
				require.Len(t, st.accountStates, accounts)
				require.NotContains(t, st.accountStates, mapKeyFromPubKey(stranger.SignKey.GetPublic()))
			}

			_, withId := signTestAclRecord(t, tc.author(fx), l.AclState().LastRecordId(), tc.content)
			require.NotPanics(t, func() { err = l.AddRawRecord(withId) })
			require.NoError(t, err)
			require.Equal(t, withId.Id, l.AclState().LastRecordId())
			check(l.AclState())

			var rebuilt AclList
			require.NotPanics(t, func() {
				rebuilt, err = BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
			})
			require.NoError(t, err)
			require.Equal(t, withId.Id, rebuilt.AclState().LastRecordId())
			check(rebuilt.AclState())
		})
	}
}

// What a record creates is keyed by its id, so a record without one is refused rather than keyed by "".
func TestAclState_ApplyRecordRequiresId(t *testing.T) {
	fx := newFixture(t)
	raw, _ := signTestAclRecord(t, fx.ownerKeys, fx.ownerAcl.AclState().LastRecordId(), requestToJoinInviteContent(t))
	rec, err := fx.ownerAcl.recordBuilder.Unmarshall(raw)
	require.NoError(t, err)
	require.NotEmpty(t, rec.Id)

	rec.Id = ""
	require.ErrorIs(t, fx.ownerAcl.AclState().Copy().ApplyRecord(rec), ErrEmptyRecordId)
}
