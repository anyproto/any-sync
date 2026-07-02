package list

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
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

func createStore(ctx context.Context, t *testing.T) anystore.DB {
	path := filepath.Join(t.TempDir(), "list.db")
	db, err := anystore.Open(ctx, path, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := db.Close()
		require.NoError(t, err)
	})
	return db
}

var mockMetadata = []byte("very important metadata")

// testNetworkKey returns a fresh random pubkey for tests that need a
// recordverifier without caring which key it trusts.
func testNetworkKey(t *testing.T) crypto.PubKey {
	_, pub, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	return pub
}

func newFixture(t *testing.T) *aclFixture {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId := "spaceId"
	ctx := context.Background()
	ownerAcl, err := newDerivedAclWithStoreProvider(spaceId, ownerKeys, []byte("metadata"), func(root *consensusproto.RawRecordWithId) (Storage, error) {
		store := createStore(ctx, t)
		headStorage, err := headstorage.New(ctx, store)
		require.NoError(t, err)
		return CreateStorage(ctx, root, headStorage, store)
	})
	require.NoError(t, err)
	accountAcl, err := newAclWithStoreProvider(ownerAcl.Root(), accountKeys, func(root *consensusproto.RawRecordWithId) (Storage, error) {
		store := createStore(ctx, t)
		headStorage, err := headstorage.New(ctx, store)
		require.NoError(t, err)
		return CreateStorage(ctx, root, headStorage, store)
	})
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
	inviteRec := listtest.WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteKey: inv.InviteKey,
		Metadata:  mockMetadata,
	})
	require.NoError(t, err)
	requestJoinRec := listtest.WrapAclRecord(requestJoin)
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
	requestAcceptRec := listtest.WrapAclRecord(requestAccept)
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
	randomAcl, err := NewInMemoryDerivedAcl("spaceId", randomKeys)
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
	inviteRec := listtest.WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building invite revoke
	inviteRevoke, err := fx.ownerAcl.RecordBuilder().BuildInviteRevoke(ownerState().lastRecordId)
	require.NoError(t, err)
	inviteRevokeRec := listtest.WrapAclRecord(inviteRevoke)
	fx.addRec(t, inviteRevokeRec)

	// checking acl state
	require.True(t, ownerState().Permissions(ownerState().pubKey).IsOwner())
	require.True(t, ownerState().Permissions(accountState().pubKey).NoPermissions())
	require.Empty(t, ownerState().invites)
	require.Empty(t, accountState().invites)
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
	inviteRec := listtest.WrapAclRecord(inv.InviteRec)
	fx.addRec(t, inviteRec)

	// building request join
	requestJoin, err := accountAcl.RecordBuilder().BuildRequestJoin(RequestJoinPayload{
		InviteKey: inv.InviteKey,
	})
	require.NoError(t, err)
	requestJoinRec := listtest.WrapAclRecord(requestJoin)
	fx.addRec(t, requestJoinRec)

	// building request decline
	requestDecline, err := ownerAcl.RecordBuilder().BuildRequestDecline(ownerState().lastRecordId)
	require.NoError(t, err)
	requestDeclineRec := listtest.WrapAclRecord(requestDecline)
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
	removeRec := listtest.WrapAclRecord(remove)
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

	_, err := BuildAclListWithIdentity(fx.accountKeys, fx.ownerAcl.storage, recordverifier.NewValidateFull())
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
	readKeyRec := listtest.WrapAclRecord(readKeyChange)
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

func TestAclList_ValidateUsesCorrectVerifier(t *testing.T) {
	fx := newFixture(t)
	var ownerAcl = fx.ownerAcl
	netKey := testNetworkKey(t)
	ownerAcl.aclState.contentValidator.(*contentValidator).verifier = recordverifier.New(netKey)
	ownerAcl.verifier = recordverifier.New(netKey)
	// building invite
	inv, err := ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)
	isCalled := false
	ok := ownerAcl.aclState.contentValidator.(*contentValidator).verifier.ShouldValidate()
	require.False(t, ok)
	err = fx.ownerAcl.ValidateRawRecord(inv.InviteRec, func(state *AclState) error {
		isCalled = true
		// check that we change the validator to the verifying one
		require.True(t, state.contentValidator.(*contentValidator).verifier.ShouldValidate())
		return nil
	})
	require.NoError(t, err)
	require.True(t, isCalled)
}

func TestAclList_ReadKeyChange(t *testing.T) {
	t.Run("success", func(t *testing.T) {
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
		readKeyRec := listtest.WrapAclRecord(readKeyChange)
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
	})
	t.Run("insufficient permissions", func(t *testing.T) {
		fx := newFixture(t)
		var (
			accountAcl = fx.accountAcl
		)
		fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer))

		newReadKey := crypto.NewAES()
		privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)

		accRecBuilder := accountAcl.RecordBuilder().(*aclRecordBuilder)
		rkChange, err := accRecBuilder.buildReadKeyChange(ReadKeyChangePayload{
			MetadataKey: privKey,
			ReadKey:     newReadKey,
		}, nil)
		require.NoError(t, err)
		content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: rkChange}}
		_, err = accRecBuilder.buildRecord(content)
		assert.ErrorIs(t, err, ErrInsufficientPermissions)

	})
}

func addReadKeyChange(t *testing.T, fx *aclFixture) string {
	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	rkChange, err := fx.ownerAcl.RecordBuilder().BuildReadKeyChange(ReadKeyChangePayload{
		MetadataKey: privKey,
		ReadKey:     newReadKey,
	})
	require.NoError(t, err)
	rec := listtest.WrapAclRecord(rkChange)
	fx.addRec(t, rec)
	return rec.Id
}

// TestAclList_ServesFullRecordsFromStorage locks in the core invariant: storage is the source of truth
// we serve to peers, and the in-memory list is only a derived view. Even if that view is shrunken (as the
// planned keep-only-ours decode-skip build will make it — dropping other members' read keys), RecordsAfter
// (the p2p/serving path) must return the full, signature-valid record read from storage, never the in-memory
// Model. Do not "optimize" RecordsAfter/RecordsBefore to serve from a.records[].Model: it would ship records
// missing every other member's read key and break the signature.
func TestAclList_ServesFullRecordsFromStorage(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Admin))
	rotId := addReadKeyChange(t, fx)

	rebuilt, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, recordverifier.NewValidateFull())
	require.NoError(t, err)

	readKeyChangeOf := func(rec *AclRecord) *aclrecordproto.AclReadKeyChange {
		for _, c := range rec.Model.(*aclrecordproto.AclData).AclContent {
			if rkc := c.GetReadKeyChange(); rkc != nil {
				return rkc
			}
		}
		return nil
	}

	// Simulate a shrunken in-memory view (what the decode-skip build will produce): drop other members'
	// read keys from the in-memory Model so it diverges from the full record in storage.
	rotRec, err := rebuilt.Get(rotId)
	require.NoError(t, err)
	readKeyChangeOf(rotRec).AccountKeys = nil
	require.Nil(t, readKeyChangeOf(rotRec).AccountKeys, "precondition: in-memory view is shrunken")

	// The serving path must ignore the view and return the rotation record in full from storage.
	served, err := rebuilt.RecordsAfter(context.Background(), "")
	require.NoError(t, err)
	var rotRaw *consensusproto.RawRecordWithId
	for _, raw := range served {
		if raw.Id == rotId {
			rotRaw = raw
		}
	}
	require.NotNil(t, rotRaw, "rotation record must be served")

	// re-decoding the served bytes also re-verifies the signature (UnmarshallWithId -> verifyRaw):
	// a record re-marshalled from the shrunken view would fail here OR carry fewer AccountKeys.
	fresh, err := rebuilt.RecordBuilder().UnmarshallWithId(rotRaw)
	require.NoError(t, err, "served record must pass signature verification")
	require.Len(t, readKeyChangeOf(fresh).AccountKeys, 2,
		"served record must carry every member's encrypted read key (owner + admin), not the shrunken view")
}

// noValidateVerifier accepts any record and disables content re-validation, modelling the trusted
// build-from-storage path (records already verified at ingest). This is the path on which the builder
// uses the keep-only-ours decode.
type noValidateVerifier struct{}

func (noValidateVerifier) VerifyAcceptor(*consensusproto.RawRecord) error { return nil }
func (noValidateVerifier) ShouldValidate() bool                           { return false }

func addAccountRemove(t *testing.T, fx *aclFixture, removed crypto.PubKey) string {
	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	remove, err := fx.ownerAcl.RecordBuilder().BuildAccountRemove(AccountRemovePayload{
		Identities: []crypto.PubKey{removed},
		Change:     ReadKeyChangePayload{MetadataKey: privKey, ReadKey: newReadKey},
	})
	require.NoError(t, err)
	rec := listtest.WrapAclRecord(remove)
	fx.addRec(t, rec)
	return rec.Id
}

func requireSameAclState(t *testing.T, want, got *AclState) {
	require.Equal(t, want.readKeyChanges, got.readKeyChanges, "readKeyChanges")
	require.Equal(t, want.lastRecordId, got.lastRecordId, "head/lastRecordId")
	require.Equal(t, len(want.keys), len(got.keys), "keys count")
	for id, wk := range want.keys {
		gk, ok := got.keys[id]
		require.True(t, ok, "missing key for %s", id)
		if wk.ReadKey != nil {
			require.NotNil(t, gk.ReadKey, "read key nil at %s", id)
			require.True(t, wk.ReadKey.Equals(gk.ReadKey), "read key mismatch at %s", id)
		} else {
			require.Nil(t, gk.ReadKey, "read key should be nil at %s", id)
		}
	}
	require.Equal(t, len(want.accountStates), len(got.accountStates), "accountStates count")
	for k, wa := range want.accountStates {
		ga, ok := got.accountStates[k]
		require.True(t, ok, "missing account state")
		require.Equal(t, wa.Permissions, ga.Permissions, "permissions")
		require.Equal(t, wa.Status, ga.Status, "status")
	}
}

// TestAclList_KeepOnlyOursBuildEquivalentState proves the keep-only-ours decode (non-validating verifier)
// produces the same derived AclState as a full decode — same read keys, accounts, invites, head — across
// rotations and a kick (which exercises the nested AclAccountRemove.readKeyChange).
func TestAclList_KeepOnlyOursBuildEquivalentState(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Admin))
	addReadKeyChange(t, fx)
	addReadKeyChange(t, fx)
	addAccountRemove(t, fx, fx.accountKeys.SignKey.GetPublic())

	full, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, recordverifier.NewValidateFull())
	require.NoError(t, err)
	partial, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
	require.NoError(t, err)

	requireSameAclState(t, full.AclState(), partial.AclState())

	// And the owner can still recover (decrypt) every historical read key after keep-only-ours.
	for id, k := range partial.AclState().Keys() {
		require.NotNil(t, k.ReadKey, "owner must recover read key at %s", id)
	}
}

// TestAclList_KeepOnlyOursBoundsResidentKeys is the fail-loud companion to the heap profile: with no
// post-build strip, a regressed decoder that kept every member's key would show up here directly. After a
// keep-only-ours build, resident accountKeys are O(rotations) (one per rotation, ours) — never
// O(rotations*members) — and strictly fewer than a full decode.
func TestAclList_KeepOnlyOursBoundsResidentKeys(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Admin)) // owner + 1 member
	const rotations = 4
	for i := 0; i < rotations; i++ {
		addReadKeyChange(t, fx)
	}

	countAccountKeys := func(l AclList) int {
		n := 0
		for _, rec := range l.Records() {
			data, ok := rec.Model.(*aclrecordproto.AclData)
			if !ok {
				continue
			}
			for _, c := range data.AclContent {
				if rkc := c.GetReadKeyChange(); rkc != nil {
					n += len(rkc.AccountKeys)
				}
				if ar := c.GetAccountRemove(); ar != nil && ar.ReadKeyChange != nil {
					n += len(ar.ReadKeyChange.AccountKeys)
				}
			}
		}
		return n
	}

	full, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, recordverifier.NewValidateFull())
	require.NoError(t, err)
	partial, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
	require.NoError(t, err)

	fullCount := countAccountKeys(full)
	partialCount := countAccountKeys(partial)
	require.LessOrEqual(t, partialCount, rotations, "keep-only-ours must be O(rotations), not O(rotations*members)")
	require.Less(t, partialCount, fullCount, "keep-only-ours must retain strictly fewer keys than a full decode")
}

// TestAclList_NonValidatingBuilderRotation guards against the decode/validate mismatch: a builder with a
// non-validating verifier (the production space-ACL build path) must NOT keep-only-ours on the Unmarshall
// path, because preflightCheck and ValidateRawRecord re-validate the decoded record with NewValidateFull,
// whose accountKeys-count check (len(accountKeys)==activeUsers) requires every active member's key. With
// keep-only-ours leaking into Unmarshall this fails with ErrIncorrectNumberOfAccounts.
func TestAclList_NonValidatingBuilderRotation(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Writer)) // owner + member = 2 active

	// Build the list with a non-validating verifier (as the production space ACL build does).
	l, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, noValidateVerifier{})
	require.NoError(t, err)

	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)

	// Blocker 1: BuildReadKeyChange runs preflightCheck (Unmarshall -> NewValidateFull).
	rkc, err := l.RecordBuilder().BuildReadKeyChange(ReadKeyChangePayload{MetadataKey: privKey, ReadKey: newReadKey})
	require.NoError(t, err, "preflightCheck must full-decode, not keep-only-ours")

	// Blocker 2: ValidateRawRecord (the coordinator path) decodes via Unmarshall -> NewValidateFull.
	require.NoError(t, l.ValidateRawRecord(rkc, nil), "ValidateRawRecord must full-decode, not keep-only-ours")
}

// TestAclList_BuildDropsRawData verifies the streaming build never retains rec.Data, while the derived state
// (read keys) stays correct — storage keeps the raw bytes and serves peers from there.
func TestAclList_BuildDropsRawData(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Admin))
	addReadKeyChange(t, fx)
	addAccountRemove(t, fx, fx.accountKeys.SignKey.GetPublic())

	rebuilt, err := BuildAclListWithIdentity(fx.ownerKeys, fx.ownerAcl.storage, recordverifier.NewValidateFull())
	require.NoError(t, err)
	for _, rec := range rebuilt.Records() {
		require.Nil(t, rec.Data, "build must not retain raw Data for %s", rec.Id)
	}
	// state intact: the current read key is still recoverable/decryptable.
	_, err = rebuilt.AclState().CurrentReadKey()
	require.NoError(t, err)
}

// TestAclList_ScanMatchesPrevIdWalk proves the GetAfterOrder 'o'-index scan yields the exact same root-first
// order as the authoritative PrevId walk for healthy storage (so the contiguity guard passes and the fast
// scan path is used), and that both drop rec.Data.
func TestAclList_ScanMatchesPrevIdWalk(t *testing.T) {
	fx := newFixture(t)
	fx.inviteAccount(t, AclPermissions(aclrecordproto.AclUserPermissions_Admin))
	addReadKeyChange(t, fx)
	addAccountRemove(t, fx, fx.accountKeys.SignKey.GetPublic())

	store := fx.ownerAcl.storage
	rb := fx.ownerAcl.recordBuilder
	head, err := store.Head(context.Background())
	require.NoError(t, err)

	scanned, err := loadRecordsByScan(context.Background(), store, rb)
	require.NoError(t, err)
	walked, err := loadRecordsByPrevId(context.Background(), store, rb, head)
	require.NoError(t, err)

	require.Equal(t, len(walked), len(scanned))
	for i := range walked {
		require.Equal(t, walked[i].Id, scanned[i].Id, "record %d id mismatch", i)
		require.Equal(t, walked[i].PrevId, scanned[i].PrevId, "record %d prevId mismatch", i)
		require.Nil(t, scanned[i].Data)
		require.Nil(t, walked[i].Data)
	}
	require.True(t, isContiguousChain(scanned, fx.ownerAcl.Id(), head), "healthy storage must pass the contiguity guard")
}

func TestAclList_IsContiguousChain(t *testing.T) {
	mk := func(id, prev string) *AclRecord { return &AclRecord{Id: id, PrevId: prev} }
	require.True(t, isContiguousChain([]*AclRecord{mk("root", ""), mk("a", "root"), mk("b", "a")}, "root", "b"))
	require.False(t, isContiguousChain(nil, "root", "head"), "empty")
	require.False(t, isContiguousChain([]*AclRecord{mk("x", ""), mk("a", "x")}, "root", "a"), "wrong root")
	require.False(t, isContiguousChain([]*AclRecord{mk("root", ""), mk("a", "root")}, "root", "b"), "wrong head")
	require.False(t, isContiguousChain([]*AclRecord{mk("root", ""), mk("a", "root"), mk("b", "wrong")}, "root", "b"), "broken link")
}

// TestAclList_ScanWorksOnInMemoryStorage is a regression guard: GetAfterOrder has different start-order
// semantics across Storage impls (inMemoryStorage requires order>=1; order 0 returns nothing). The scan must
// return the full in-memory log, not silently fall back. With GetAfterOrder(0) this returned empty.
func TestAclList_ScanWorksOnInMemoryStorage(t *testing.T) {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	acl, err := NewInMemoryDerivedAcl("spaceId", keys)
	require.NoError(t, err)
	al := acl.(*aclList)

	newReadKey := crypto.NewAES()
	privKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	rkc, err := al.RecordBuilder().BuildReadKeyChange(ReadKeyChangePayload{MetadataKey: privKey, ReadKey: newReadKey})
	require.NoError(t, err)
	require.NoError(t, al.AddRawRecord(listtest.WrapAclRecord(rkc)))

	head, err := al.storage.Head(context.Background())
	require.NoError(t, err)
	scanned, err := loadRecordsByScan(context.Background(), al.storage, al.recordBuilder)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(scanned), 2, "scan must return the full in-memory log (root + rotation), not empty")
	require.Equal(t, al.Id(), scanned[0].Id, "root first")
	require.True(t, isContiguousChain(scanned, al.Id(), head), "scan path must be used (contiguity holds) for in-memory storage")
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
	permissionChangeRec := listtest.WrapAclRecord(permissionChange)
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
	removeRequestRec := listtest.WrapAclRecord(removeRequest)
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
	removeRec := listtest.WrapAclRecord(remove)
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

func TestAclPermissions_IsAdmin(t *testing.T) {
	require.True(t, AclPermissionsAdmin.IsAdmin())
	require.False(t, AclPermissionsOwner.IsAdmin())
	require.False(t, AclPermissionsWriter.IsAdmin())
	require.False(t, AclPermissionsReader.IsAdmin())
	require.False(t, AclPermissionsGuest.IsAdmin())
	require.False(t, AclPermissionsNone.IsAdmin())
}

// FR1: only the owner can grant the Admin role.
func TestAclList_AdminCannotGrantAdmin(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []struct {
		cmd string
		err error
	}{
		{"a.init::a", nil},
		{"a.invite::invId", nil},
		{"b.join::invId", nil},
		{"a.approve::b,r", nil},
		{"e.join::invId", nil},
		{"a.approve::e,adm", nil},
		// e is admin; cannot grant admin to b
		{"e.changes::b,adm", ErrInsufficientPermissions},
	}
	for _, c := range cmds {
		require.Equal(t, c.err, a.Execute(c.cmd), c.cmd)
	}
}

// FR2: only the owner can revoke the Admin role.
func TestAclList_AdminCannotRevokeAdmin(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []struct {
		cmd string
		err error
	}{
		{"a.init::a", nil},
		{"a.invite::invId", nil},
		{"e.join::invId", nil},
		{"a.approve::e,adm", nil},
		{"y.join::invId", nil},
		{"a.approve::y,adm", nil},
		// e is admin; cannot demote y from admin
		{"e.changes::y,rw", ErrInsufficientPermissions},
		// owner can demote
		{"a.changes::y,rw", nil},
	}
	for _, c := range cmds {
		require.Equal(t, c.err, a.Execute(c.cmd), c.cmd)
	}
}

// Owner can grant and revoke Admin freely.
func TestAclList_OwnerCanGrantAndRevokeAdmin(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []struct {
		cmd string
		err error
	}{
		{"a.init::a", nil},
		{"a.invite::invId", nil},
		{"b.join::invId", nil},
		{"a.approve::b,r", nil},
		// owner promotes b to admin
		{"a.changes::b,adm", nil},
		// owner demotes b back to writer
		{"a.changes::b,rw", nil},
	}
	for _, c := range cmds {
		require.Equal(t, c.err, a.Execute(c.cmd), c.cmd)
	}
}

// FR1: only the owner can introduce a new Admin via AccountsAdd.
func TestAclList_AdminCannotAddNewAdmin(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []struct {
		cmd string
		err error
	}{
		{"a.init::a", nil},
		{"a.invite::invId", nil},
		{"e.join::invId", nil},
		{"a.approve::e,adm", nil},
		// e (admin) can add a writer
		{"e.add::x,rw,m1", nil},
		// e (admin) cannot add an admin
		{"e.add::z,adm,m2", ErrInsufficientPermissions},
		// owner can
		{"a.add::z,adm,m2", nil},
	}
	for _, c := range cmds {
		require.Equal(t, c.err, a.Execute(c.cmd), c.cmd)
	}
}

// FR1: only the owner can issue or change an invite that grants Admin.
func TestAclList_AdminCannotIssueAdminInvite(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []struct {
		cmd string
		err error
	}{
		{"a.init::a", nil},
		{"a.invite::invId", nil},
		{"e.join::invId", nil},
		{"a.approve::e,adm", nil},
		// e (admin) cannot create an anyone-can-join invite at Admin level
		{"e.invite_anyone::admInv,a", ErrInsufficientPermissions},
		// owner can
		{"a.invite_anyone::admInv,a", nil},
		// e (admin) cannot change an existing invite to Admin level
		{"a.invite_anyone::otherInv,rw", nil},
		{"e.invite_change::otherInv,a", ErrInsufficientPermissions},
		// owner can
		{"a.invite_change::otherInv,a", nil},
	}
	for _, c := range cmds {
		require.Equal(t, c.err, a.Execute(c.cmd), c.cmd)
	}
}

// FR1: only the owner can accept a join request at Admin level.
func TestAclList_AdminCannotApproveAsAdmin(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []struct {
		cmd string
		err error
	}{
		{"a.init::a", nil},
		{"a.invite::invId", nil},
		{"e.join::invId", nil},
		{"a.approve::e,adm", nil},
		{"b.join::invId", nil},
		// e (admin) cannot approve b as admin
		{"e.approve::b,adm", ErrInsufficientPermissions},
		// owner can
		{"a.approve::b,adm", nil},
	}
	for _, c := range cmds {
		require.Equal(t, c.err, a.Execute(c.cmd), c.cmd)
	}
}

// FR2: only the owner can remove an Admin (revoking via account removal).
func TestAclList_AdminCannotRemoveAdmin(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []struct {
		cmd string
		err error
	}{
		{"a.init::a", nil},
		{"a.invite::invId", nil},
		{"e.join::invId", nil},
		{"a.approve::e,adm", nil},
		{"y.join::invId", nil},
		{"a.approve::y,adm", nil},
		{"b.join::invId", nil},
		{"a.approve::b,rw", nil},
		// e (admin) can remove a regular writer
		{"e.remove::b", nil},
		// e (admin) cannot remove another admin
		{"e.remove::y", ErrInsufficientPermissions},
		// owner can remove the admin
		{"a.remove::y", nil},
	}
	for _, c := range cmds {
		require.Equal(t, c.err, a.Execute(c.cmd), c.cmd)
	}
}
