package list

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
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

// requestJoinContent returns the content of a valid join request by the fixture's account, made through a
// request-to-join invite the owner has just added.
func requestJoinContent(t *testing.T, fx *aclFixture) *aclrecordproto.AclContentValue {
	inv, err := fx.ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	fx.addRec(t, listtest.WrapAclRecord(inv.InviteRec))

	joinRaw, err := fx.accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{InviteKey: inv.InviteKey, Metadata: mockMetadata})
	require.NoError(t, err)
	rec := &consensusproto.Record{}
	require.NoError(t, rec.UnmarshalVT(joinRaw.Payload))
	data := &aclrecordproto.AclData{}
	require.NoError(t, data.UnmarshalVT(rec.Data))
	require.Len(t, data.AclContent, 1)
	return data.AclContent[0]
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

// An empty request or invite reference names the record carrying it, so a value can refer to what an earlier
// value in the same record created. The record must come out the same at admission, on replay and on
// rebuild, whichever verifier replays it.
func TestAclList_SelfReferenceResolvesToRecord(t *testing.T) {
	verifiers := map[string]recordverifier.AcceptorVerifier{
		"validating":     recordverifier.NewValidateFull(),
		"non-validating": noValidateVerifier{},
	}
	for name, tc := range map[string]struct {
		record func(t *testing.T, fx *aclFixture) (*consensusproto.RawRecord, *consensusproto.RawRecordWithId)
		check  func(t *testing.T, fx *aclFixture, st *AclState)
	}{
		"join request cancelled in the same record": {
			record: func(t *testing.T, fx *aclFixture) (*consensusproto.RawRecord, *consensusproto.RawRecordWithId) {
				join := requestJoinContent(t, fx)
				cancel := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestCancel{
					RequestCancel: &aclrecordproto.AclAccountRequestCancel{},
				}}
				return signTestAclRecord(t, fx.accountKeys, fx.ownerAcl.AclState().LastRecordId(), join, cancel)
			},
			check: func(t *testing.T, fx *aclFixture, st *AclState) {
				joiner := st.accountStates[mapKeyFromPubKey(fx.accountKeys.SignKey.GetPublic())]
				require.Equal(t, StatusCanceled, joiner.Status)
				require.True(t, joiner.Permissions.NoPermissions())
				require.Empty(t, st.pendingRequests)
				require.Empty(t, st.requestRecords)
			},
		},
		"invite revoked in the same record": {
			record: func(t *testing.T, fx *aclFixture) (*consensusproto.RawRecord, *consensusproto.RawRecordWithId) {
				revoke := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteRevoke{
					InviteRevoke: &aclrecordproto.AclAccountInviteRevoke{},
				}}
				return signTestAclRecord(t, fx.ownerKeys, fx.ownerAcl.AclState().LastRecordId(), requestToJoinInviteContent(t), revoke)
			},
			check: func(t *testing.T, fx *aclFixture, st *AclState) {
				require.Empty(t, st.invites)
			},
		},
	} {
		for vName, verifier := range verifiers {
			t.Run(name+"/"+vName, func(t *testing.T) {
				fx := newFixture(t)
				raw, withId := tc.record(t, fx)

				require.NoError(t, fx.ownerAcl.ValidateRawRecord(raw, func(st *AclState) error {
					tc.check(t, fx, st)
					return nil
				}))

				l, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, verifier)
				require.NoError(t, err)
				require.NoError(t, l.AddRawRecord(withId))
				require.Equal(t, withId.Id, l.AclState().LastRecordId())
				tc.check(t, fx, l.AclState())

				rebuilt, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, verifier)
				require.NoError(t, err)
				require.Equal(t, withId.Id, rebuilt.AclState().LastRecordId())
				tc.check(t, fx, rebuilt.AclState())
			})
		}
	}
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
			author: func(*aclFixture) *accountdata.AccountKeys { return stranger },
			content: &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestCancel{
				RequestCancel: &aclrecordproto.AclAccountRequestCancel{RecordId: "missing"},
			}},
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
			check := func(st *AclState) {
				require.Empty(t, st.invites)
				require.Empty(t, st.requestRecords)
				require.Empty(t, st.pendingRequests)
				require.True(t, st.Permissions(stranger.SignKey.GetPublic()).NoPermissions())
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
