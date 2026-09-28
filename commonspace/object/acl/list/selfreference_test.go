package list

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

// requestJoinContent returns the content of a valid join request by the fixture's account, made through a
// request-to-join invite the owner has just added.
func requestJoinContent(t *testing.T, fx *aclFixture) *aclrecordproto.AclContentValue {
	inv, err := fx.ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	invRec := listtest.WrapAclRecord(inv.InviteRec)
	require.NoError(t, fx.ownerAcl.AddRawRecord(invRec))
	require.NoError(t, fx.accountAcl.AddRawRecord(invRec))

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

// Before the network accepts a record it has no id, so what it creates must not be reachable by its own
// content: a later value resolving against an earlier one would pass admission, then fail to resolve on
// replay once the record carries its real id.
func TestAclList_ValidateRawRecordRejectsSelfReference(t *testing.T) {
	t.Run("join request cancelled in the same record", func(t *testing.T) {
		fx := newFixture(t)
		join := requestJoinContent(t, fx)
		cancel := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestCancel{
			RequestCancel: &aclrecordproto.AclAccountRequestCancel{},
		}}
		raw, _ := signTestAclRecord(t, fx.accountKeys, fx.ownerAcl.AclState().LastRecordId(), join, cancel)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(raw, nil), ErrNoSuchRequest)
	})

	t.Run("invite revoked in the same record", func(t *testing.T) {
		fx := newFixture(t)
		revoke := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteRevoke{
			InviteRevoke: &aclrecordproto.AclAccountInviteRevoke{},
		}}
		raw, _ := signTestAclRecord(t, fx.ownerKeys, fx.ownerAcl.AclState().LastRecordId(), requestToJoinInviteContent(t), revoke)
		require.ErrorIs(t, fx.ownerAcl.ValidateRawRecord(raw, nil), ErrNoSuchInvite)
	})
}

// A network client does not validate content, so a record already in the log whose request or invite
// reference does not resolve (such as one admitted before self-references were refused) must apply as a
// no-op, not panic or plant a half-built entry.
func TestAclList_ReplayUnresolvedReference(t *testing.T) {
	stranger, err := accountdata.NewRandom()
	require.NoError(t, err)
	strangerIdentity, err := stranger.SignKey.GetPublic().Marshall()
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		author  func(fx *aclFixture) *accountdata.AccountKeys
		content func(t *testing.T, fx *aclFixture) []*aclrecordproto.AclContentValue
	}{
		"join request cancelled in the same record": {
			author: func(fx *aclFixture) *accountdata.AccountKeys { return fx.accountKeys },
			content: func(t *testing.T, fx *aclFixture) []*aclrecordproto.AclContentValue {
				return []*aclrecordproto.AclContentValue{requestJoinContent(t, fx), {Value: &aclrecordproto.AclContentValue_RequestCancel{
					RequestCancel: &aclrecordproto.AclAccountRequestCancel{},
				}}}
			},
		},
		"request cancel": {
			author: func(*aclFixture) *accountdata.AccountKeys { return stranger },
			content: func(*testing.T, *aclFixture) []*aclrecordproto.AclContentValue {
				return []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_RequestCancel{
					RequestCancel: &aclrecordproto.AclAccountRequestCancel{RecordId: "missing"},
				}}}
			},
		},
		"request decline": {
			author: func(fx *aclFixture) *accountdata.AccountKeys { return fx.ownerKeys },
			content: func(*testing.T, *aclFixture) []*aclrecordproto.AclContentValue {
				return []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_RequestDecline{
					RequestDecline: &aclrecordproto.AclAccountRequestDecline{RequestRecordId: "missing"},
				}}}
			},
		},
		"request accept": {
			author: func(fx *aclFixture) *accountdata.AccountKeys { return fx.ownerKeys },
			content: func(*testing.T, *aclFixture) []*aclrecordproto.AclContentValue {
				return []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_RequestAccept{
					RequestAccept: &aclrecordproto.AclAccountRequestAccept{
						Identity:        strangerIdentity,
						RequestRecordId: "missing",
						Permissions:     aclrecordproto.AclUserPermissions_Writer,
					},
				}}}
			},
		},
		"invite change": {
			author: func(fx *aclFixture) *accountdata.AccountKeys { return fx.ownerKeys },
			content: func(*testing.T, *aclFixture) []*aclrecordproto.AclContentValue {
				return []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_InviteChange{
					InviteChange: &aclrecordproto.AclAccountInviteChange{
						InviteRecordId: "missing",
						Permissions:    aclrecordproto.AclUserPermissions_Writer,
					},
				}}}
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newFixture(t)
			content := tc.content(t, fx)
			l, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
			require.NoError(t, err)
			invites := len(l.AclState().Invites())

			_, withId := signTestAclRecord(t, tc.author(fx), l.AclState().LastRecordId(), content...)
			require.NotPanics(t, func() { err = l.AddRawRecord(withId) })
			require.NoError(t, err)
			require.Equal(t, withId.Id, l.AclState().LastRecordId())
			require.Len(t, l.AclState().Invites(), invites)
			require.True(t, l.AclState().Permissions(stranger.SignKey.GetPublic()).NoPermissions())

			var rebuilt AclList
			require.NotPanics(t, func() {
				rebuilt, err = BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
			})
			require.NoError(t, err)
			require.Equal(t, withId.Id, rebuilt.AclState().LastRecordId())
		})
	}
}
