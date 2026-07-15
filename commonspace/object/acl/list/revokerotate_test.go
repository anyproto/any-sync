package list

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/util/crypto"
)

// TestAclList_BuildBatchRequest_RevokeAndRotate covers the new BatchRequestPayload.ReadKeyChange path:
// one record that revokes invites and rotates the read key, with the new key withheld from the invites
// being revoked in that same record.
func TestAclList_BuildBatchRequest_RevokeAndRotate(t *testing.T) {
	readKeyChangeContent := func(t *testing.T, acl *aclList, rec *aclrecordproto.AclData) *aclrecordproto.AclReadKeyChange {
		t.Helper()
		var rkc *aclrecordproto.AclReadKeyChange
		for _, c := range rec.AclContent {
			if v := c.GetReadKeyChange(); v != nil {
				rkc = v
			}
		}
		return rkc
	}

	t.Run("revokes the invite and rotates the read key, excluding the revoked invite", func(t *testing.T) {
		fx := newFixture(t)
		fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))
		var (
			ownerAcl   = fx.ownerAcl
			accountAcl = fx.accountAcl
			ownerState = func() *AclState { return ownerAcl.aclState }
		)

		inv, err := ownerAcl.RecordBuilder().BuildInviteAnyone(AclPermissions(aclrecordproto.AclUserPermissions_Reader))
		require.NoError(t, err)
		fx.addRec(t, listtest.WrapAclRecord(inv.InviteRec))
		inviteKey := inv.InviteKey

		oldReadKey, err := ownerState().CurrentReadKey()
		require.NoError(t, err)

		newReadKey := crypto.NewAES()
		newMetaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)

		res, err := ownerAcl.RecordBuilder().BuildBatchRequest(BatchRequestPayload{
			InviteRevokes: ownerState().InviteIds(),
			ReadKeyChange: &ReadKeyChangePayload{MetadataKey: newMetaKey, ReadKey: newReadKey},
		})
		require.NoError(t, err)
		rec := listtest.WrapAclRecord(res.Rec)

		// the built record carries both a revoke and a rotation, and the rotation covers the two members
		// (owner + writer) but none of the revoked invites
		parsed, err := ownerAcl.RecordBuilder().Unmarshall(res.Rec)
		require.NoError(t, err)
		var sawRevoke bool
		for _, c := range parsed.Model.(*aclrecordproto.AclData).AclContent {
			if c.GetInviteRevoke() != nil {
				sawRevoke = true
			}
		}
		require.True(t, sawRevoke, "record must revoke the invite")
		rkc := readKeyChangeContent(t, ownerAcl, parsed.Model.(*aclrecordproto.AclData))
		require.NotNil(t, rkc, "record must rotate the read key")
		require.Empty(t, rkc.InviteKeys, "the new key is not re-encrypted for the revoked invite")
		require.Len(t, rkc.AccountKeys, 2, "the new key is re-encrypted for owner + member")

		// the coordinator's admission path accepts it
		require.NoError(t, ownerAcl.ValidateRawRecord(res.Rec, nil))

		// applying it: invite gone, read key rotated, member keeps access, invite key locked out
		fx.addRec(t, rec)
		require.Empty(t, ownerState().Invites(aclrecordproto.AclInviteType_AnyoneCanJoin))
		cur, err := ownerState().CurrentReadKey()
		require.NoError(t, err)
		require.True(t, cur.Equals(newReadKey))
		require.False(t, cur.Equals(oldReadKey))
		require.True(t, accountAcl.aclState.keys[rec.Id].ReadKey.Equals(newReadKey))
		_, err = ownerState().decryptReadKeyFromInviteWithoutApprove(inviteKey)
		require.Error(t, err)
	})

	t.Run("a surviving invite stays in the new key while the revoked one is excluded", func(t *testing.T) {
		fx := newFixture(t)
		var (
			ownerAcl   = fx.ownerAcl
			ownerState = func() *AclState { return ownerAcl.aclState }
		)

		invRevoked, err := ownerAcl.RecordBuilder().BuildInviteAnyone(AclPermissions(aclrecordproto.AclUserPermissions_Reader))
		require.NoError(t, err)
		fx.addRec(t, listtest.WrapAclRecord(invRevoked.InviteRec))
		invKept, err := ownerAcl.RecordBuilder().BuildInviteAnyone(AclPermissions(aclrecordproto.AclUserPermissions_Reader))
		require.NoError(t, err)
		fx.addRec(t, listtest.WrapAclRecord(invKept.InviteRec))

		revokedId, err := ownerState().GetInviteIdByPrivKey(invRevoked.InviteKey)
		require.NoError(t, err)
		keptId, err := ownerState().GetInviteIdByPrivKey(invKept.InviteKey)
		require.NoError(t, err)

		newReadKey := crypto.NewAES()
		newMetaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)

		res, err := ownerAcl.RecordBuilder().BuildBatchRequest(BatchRequestPayload{
			InviteRevokes: []string{revokedId},
			ReadKeyChange: &ReadKeyChangePayload{MetadataKey: newMetaKey, ReadKey: newReadKey},
		})
		require.NoError(t, err)

		require.NoError(t, ownerAcl.ValidateRawRecord(res.Rec, nil))
		fx.addRec(t, listtest.WrapAclRecord(res.Rec))

		// only the kept invite remains, and it can derive the rotated key; the revoked one is gone
		remaining := ownerState().Invites(aclrecordproto.AclInviteType_AnyoneCanJoin)
		require.Len(t, remaining, 1)
		require.Equal(t, keptId, remaining[0].Id)
		derived, err := ownerState().decryptReadKeyFromInviteWithoutApprove(invKept.InviteKey)
		require.NoError(t, err)
		require.True(t, derived.Equals(newReadKey))
		_, err = ownerState().decryptReadKeyFromInviteWithoutApprove(invRevoked.InviteKey)
		require.Error(t, err)
	})

	t.Run("rejects rotation alongside a membership or invite change", func(t *testing.T) {
		fx := newFixture(t)
		newReadKey := crypto.NewAES()
		newMetaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		_, err = fx.ownerAcl.RecordBuilder().BuildBatchRequest(BatchRequestPayload{
			NewInvites:    []AclPermissions{AclPermissions(aclrecordproto.AclUserPermissions_Reader)},
			ReadKeyChange: &ReadKeyChangePayload{MetadataKey: newMetaKey, ReadKey: newReadKey},
		})
		require.ErrorIs(t, err, ErrReadKeyChangeNotAlone)
	})
}

// TestAclList_RevokeAndRotate_CurrentCodeCompat pins that a revoke+rotate record is admissible on
// unmodified any-sync: the record is assembled from primitives that predate this change and is accepted
// by both the coordinator's admission path (ValidateRawRecord) and the client apply path, because both
// validate by applying it in order — the revoke lands first, so the invite is gone from the state by
// the time the rotation is checked. The frozen ValidateAclRecordContents would reject it, but nothing on
// the admission path calls it. So only the client that produces the record needs this version.
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
	rkChange, err := rb.buildReadKeyChange(ReadKeyChangePayload{MetadataKey: newMetaKey, ReadKey: newReadKey}, nil, nil)
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
