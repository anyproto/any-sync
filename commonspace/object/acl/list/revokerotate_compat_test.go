package list

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/util/crypto"
)

// TestAclList_RevokeAndRotate_CurrentCodeCompat locks in the backward-compatibility guarantee for the
// planned "revoke an anyone-can-join invite and rotate the read key in one record" feature.
//
// An anyone-can-join invite embeds the current read key encrypted to the invite key, so revoking it is
// not enough — anyone who saved the link keeps the read key. The fix rotates the read key in the same
// record, re-encrypting a new key for every remaining member and invite but NOT for the revoked one.
//
// This test builds exactly that record shape using only primitives that exist today, and asserts two
// things about UNMODIFIED any-sync:
//
//  1. The client apply path (AddRawRecord -> ApplyRecord -> applyChangeData) accepts and applies it,
//     because contents are applied in order: the revoke lands first, so by the time the read-key change
//     is applied the invite is already gone from the state and the "new key must cover every invite"
//     check is satisfied. Old clients therefore need no upgrade.
//
//  2. The frozen record-level validation (ValidateAclRecordContents, which the coordinator/acceptor runs
//     against the pre-record state) rejects it, because there the invite is still present and the new
//     key omits it. That is the single place that must be taught to exclude same-record revoked invites.
func TestAclList_RevokeAndRotate_CurrentCodeCompat(t *testing.T) {
	fx := newFixture(t)
	// a non-owner member, so the rotation has to re-encrypt the new key for someone other than the owner
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))

	var (
		ownerAcl   = fx.ownerAcl
		accountAcl = fx.accountAcl
		ownerState = func() *AclState { return ownerAcl.aclState }
	)

	// owner creates an anyone-can-join invite — the kind that leaks the read key and thus needs rotation
	inv, err := ownerAcl.RecordBuilder().BuildInviteAnyone(AclPermissions(aclrecordproto.AclUserPermissions_Reader))
	require.NoError(t, err)
	inviteRec := listtest.WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)
	inviteId := inviteRec.Id
	inviteKey := inv.InviteKey

	require.Len(t, ownerState().Invites(aclrecordproto.AclInviteType_AnyoneCanJoin), 1)
	oldReadKey, err := ownerState().CurrentReadKey()
	require.NoError(t, err)
	// the invite key really can derive the current read key — the exposure rotation must close
	derived, err := ownerState().decryptReadKeyFromInviteWithoutApprove(inviteKey)
	require.NoError(t, err)
	require.True(t, derived.Equals(oldReadKey))

	// --- assemble the record the feature will emit: [revoke(inviteId), readKeyChange excluding inviteId]
	newReadKey := crypto.NewAES()
	newMetaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)

	// Build the read-key-change content against a POST-revoke state so it naturally excludes the invite,
	// using the existing (unchanged) builder. Derive that state in memory (copy + drop the invite) rather
	// than persisting a revoke, which would pollute the shared storage. This is the exact content the new
	// builder will produce.
	postRevoke := ownerState().Copy()
	delete(postRevoke.invites, inviteId)
	rb := NewAclRecordBuilder(ownerAcl.Id(), crypto.NewKeyStorage(), fx.ownerKeys, recordverifier.NewValidateFull()).(*aclRecordBuilder)
	rb.state = postRevoke
	rkChange, err := rb.buildReadKeyChange(ReadKeyChangePayload{MetadataKey: newMetaKey, ReadKey: newReadKey}, nil)
	require.NoError(t, err)
	// the new key is NOT re-encrypted for the revoked invite
	require.Empty(t, rkChange.InviteKeys)

	ob := ownerAcl.RecordBuilder().(*aclRecordBuilder)
	revokeContent, err := ob.buildInviteRevoke(inviteId)
	require.NoError(t, err)
	rkContent := &aclrecordproto.AclContentValue{
		Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: rkChange},
	}
	// revoke BEFORE rotate — this ordering is what makes the apply path accept it
	combined, err := ob.buildRecords([]*aclrecordproto.AclContentValue{revokeContent, rkContent})
	require.NoError(t, err)
	combinedRec := listtest.WrapAclRecord(combined)

	// (1) the coordinator admits a newly-submitted record with ValidateRawRecord, which validates by
	// APPLYING it in order (revoke then rotate) — not with the frozen ValidateAclRecordContents. So it
	// ACCEPTS this record on current, unmodified any-sync: by the time the read-key change is applied the
	// invite is already gone from the state. No coordinator update is required.
	require.NoError(t, ownerAcl.ValidateRawRecord(combined, nil))

	// For contrast only: the frozen, order-independent ValidateAclRecordContents WOULD reject it (there
	// the invite is still present, so the new key looks like it omits an invite). That path is on no live
	// admission flow — neither clients (apply path) nor the coordinator (ValidateRawRecord -> ApplyRecord)
	// call it — so it does not gate this feature. Asserted here to pin exactly why the earlier
	// "coordinator must change" reasoning was wrong.
	parsed, err := ownerAcl.RecordBuilder().Unmarshall(combined)
	require.NoError(t, err)
	frozen := newContentValidator(ownerState().keyStore, ownerState(), recordverifier.NewValidateFull())
	require.ErrorIs(t, frozen.ValidateAclRecordContents(parsed), ErrIncorrectNumberOfAccounts)

	// (2) the client apply path accepts and applies it on current, unmodified any-sync — for the owner AND
	// the member (addRec applies to both and fails the test on any error).
	fx.addRec(t, combinedRec)

	// the invite is gone and the read key has rotated
	require.Empty(t, ownerState().Invites(aclrecordproto.AclInviteType_AnyoneCanJoin))
	cur, err := ownerState().CurrentReadKey()
	require.NoError(t, err)
	require.True(t, cur.Equals(newReadKey))
	require.False(t, cur.Equals(oldReadKey))
	// the surviving member was re-encrypted the new key and keeps access
	require.True(t, accountAcl.aclState.keys[combinedRec.Id].ReadKey.Equals(newReadKey))
	// the revoked invite key can no longer obtain a read key: it is gone from the acl entirely
	_, err = ownerState().decryptReadKeyFromInviteWithoutApprove(inviteKey)
	require.Error(t, err)
}
