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

// newChildAcl builds a child-space acl whose root pins the given legal owner
func newChildAcl(t *testing.T, parentSpaceId string, legalOwner crypto.PubKey) (*accountdata.AccountKeys, AclList) {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	root, err := newTestAclRecordBuilder(keys).BuildRoot(RootContent{
		PrivKey:       keys.SignKey,
		MasterKey:     masterKey,
		Change:        newTestReadKeyChangePayload(),
		Metadata:      []byte("m"),
		ParentSpaceId: parentSpaceId,
		LegalOwner:    legalOwner,
	})
	require.NoError(t, err)
	acl, err := newInMemoryAclWithRoot(keys, root)
	require.NoError(t, err)
	return keys, acl
}

// buildAclRecordSignedBy assembles a raw acl record with one content value, signed by the given keys.
// Unlike the record builder it allows an author who holds no permissions in the list.
func buildAclRecordSignedBy(t *testing.T, prevId string, keys *accountdata.AccountKeys, content *aclrecordproto.AclContentValue) *consensusproto.RawRecordWithId {
	data := &aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{content}}
	marshalledData, err := data.MarshalVT()
	require.NoError(t, err)
	protoKey, err := keys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	rec := &consensusproto.Record{
		PrevId:    prevId,
		Identity:  protoKey,
		Data:      marshalledData,
		Timestamp: time.Now().Unix(),
	}
	marshalledRec, err := rec.MarshalVT()
	require.NoError(t, err)
	sig, err := keys.SignKey.Sign(marshalledRec)
	require.NoError(t, err)
	return listtest.WrapAclRecord(&consensusproto.RawRecord{Payload: marshalledRec, Signature: sig})
}

// makeOwnershipProof produces raw parent-acl record bytes carrying one AclOwnershipChange signed by signer
func makeOwnershipProof(t *testing.T, signer *accountdata.AccountKeys, newOwner crypto.PubKey) []byte {
	newOwnerProto, err := newOwner.Marshall()
	require.NoError(t, err)
	content := &aclrecordproto.AclContentValue{
		Value: &aclrecordproto.AclContentValue_OwnershipChange{
			OwnershipChange: &aclrecordproto.AclOwnershipChange{
				NewOwnerIdentity:    newOwnerProto,
				OldOwnerPermissions: aclrecordproto.AclUserPermissions_Admin,
			},
		},
	}
	rawWithId := buildAclRecordSignedBy(t, "parent-prev", signer, content)
	return rawWithId.Payload
}

func legalOwnerUpdateContent(proofs ...[]byte) *aclrecordproto.AclContentValue {
	return &aclrecordproto.AclContentValue{
		Value: &aclrecordproto.AclContentValue_LegalOwnerUpdate{
			LegalOwnerUpdate: &aclrecordproto.AclLegalOwnerUpdate{OwnershipChanges: proofs},
		},
	}
}

func TestNestedSpaces_ChildRoot(t *testing.T) {
	parentOwner, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	_, acl := newChildAcl(t, "parent.id", parentOwner.GetPublic())
	st := acl.AclState()
	require.True(t, st.IsChildSpace())
	require.Equal(t, "parent.id", st.ParentSpaceId())
	require.True(t, st.LegalOwner().Equals(parentOwner.GetPublic()))
}

func TestNestedSpaces_ChildRootBothOrNeither(t *testing.T) {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	_, err = newTestAclRecordBuilder(keys).BuildRoot(RootContent{
		PrivKey:       keys.SignKey,
		MasterKey:     masterKey,
		Change:        newTestReadKeyChangePayload(),
		Metadata:      []byte("m"),
		ParentSpaceId: "parent.id",
	})
	require.ErrorIs(t, err, ErrIncorrectRoot)
}

func TestNestedSpaces_ChildRegister(t *testing.T) {
	a := NewAclExecutor("spaceId")
	for _, cmd := range []string{
		"a.init::a",
		"a.invite::inv",
		"b.join::inv",
		"a.approve::b,rw",
	} {
		require.NoError(t, a.Execute(cmd))
	}
	ownerAcl := a.ActualAccounts()["a"].Acl

	reg, err := ownerAcl.RecordBuilder().BuildChildRegister(ChildRegisterPayload{
		ChildSpaceId:   "child.1",
		ChildAclRootId: "childroot1",
		OrgPermission:  AclPermissionsNone,
	})
	require.NoError(t, err)
	require.NoError(t, ownerAcl.AddRawRecord(listtest.WrapAclRecord(reg)))

	st := ownerAcl.AclState()
	registration, ok := st.ChildRegistration("child.1")
	require.True(t, ok)
	require.Equal(t, "childroot1", registration.ChildAclRootId)
	require.False(t, registration.Revoked)
	require.Len(t, st.ChildRegistrations(), 1)

	// duplicate registration is rejected at build (preflight validation)
	_, err = ownerAcl.RecordBuilder().BuildChildRegister(ChildRegisterPayload{
		ChildSpaceId:   "child.1",
		ChildAclRootId: "childroot2",
	})
	require.ErrorIs(t, err, ErrChildAlreadyRegistered)

	// org cannot grant itself ownership of the child
	_, err = ownerAcl.RecordBuilder().BuildChildRegister(ChildRegisterPayload{
		ChildSpaceId:   "child.2",
		ChildAclRootId: "childroot2",
		OrgPermission:  AclPermissionsOwner,
	})
	require.ErrorIs(t, err, ErrIsOwner)

	// revoke, then re-register
	revoke, err := ownerAcl.RecordBuilder().BuildChildRegisterRevoke("child.1")
	require.NoError(t, err)
	require.NoError(t, ownerAcl.AddRawRecord(listtest.WrapAclRecord(revoke)))
	registration, ok = ownerAcl.AclState().ChildRegistration("child.1")
	require.True(t, ok)
	require.True(t, registration.Revoked)

	_, err = ownerAcl.RecordBuilder().BuildChildRegisterRevoke("child.1")
	require.ErrorIs(t, err, ErrNoSuchChildRegistration)

	reg2, err := ownerAcl.RecordBuilder().BuildChildRegister(ChildRegisterPayload{
		ChildSpaceId:   "child.1",
		ChildAclRootId: "childroot3",
	})
	require.NoError(t, err)
	require.NoError(t, ownerAcl.AddRawRecord(listtest.WrapAclRecord(reg2)))
	registration, _ = ownerAcl.AclState().ChildRegistration("child.1")
	require.Equal(t, "childroot3", registration.ChildAclRootId)
	require.False(t, registration.Revoked)
}

func TestNestedSpaces_ChildRegisterRequiresAdmin(t *testing.T) {
	a := NewAclExecutor("spaceId")
	for _, cmd := range []string{
		"a.init::a",
		"a.invite::inv",
		"b.join::inv",
		"a.approve::b,rw",
	} {
		require.NoError(t, a.Execute(cmd))
	}
	var (
		ownerAcl   = a.ActualAccounts()["a"].Acl
		writerKeys = a.ActualAccounts()["b"].Keys
	)
	content := &aclrecordproto.AclContentValue{
		Value: &aclrecordproto.AclContentValue_ChildRegister{
			ChildRegister: &aclrecordproto.AclChildRegister{
				ChildSpaceId:   "child.1",
				ChildAclRootId: "childroot1",
			},
		},
	}
	rec := buildAclRecordSignedBy(t, ownerAcl.Head().Id, writerKeys, content)
	require.ErrorIs(t, ownerAcl.AddRawRecord(rec), ErrInsufficientPermissions)
}

func TestNestedSpaces_LegalOwnerUpdate(t *testing.T) {
	aliceKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	bobKeys, err := accountdata.NewRandom()
	require.NoError(t, err)

	_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())

	proof := makeOwnershipProof(t, aliceKeys, bobKeys.SignKey.GetPublic())
	update := buildAclRecordSignedBy(t, childAcl.Head().Id, bobKeys, legalOwnerUpdateContent(proof))
	require.NoError(t, childAcl.AddRawRecord(update))
	require.True(t, childAcl.AclState().LegalOwner().Equals(bobKeys.SignKey.GetPublic()))
}

func TestNestedSpaces_LegalOwnerUpdateMultiHop(t *testing.T) {
	aliceKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	bobKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	carolKeys, err := accountdata.NewRandom()
	require.NoError(t, err)

	_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())

	proofs := [][]byte{
		makeOwnershipProof(t, aliceKeys, bobKeys.SignKey.GetPublic()),
		makeOwnershipProof(t, bobKeys, carolKeys.SignKey.GetPublic()),
	}
	update := buildAclRecordSignedBy(t, childAcl.Head().Id, carolKeys, legalOwnerUpdateContent(proofs...))
	require.NoError(t, childAcl.AddRawRecord(update))
	require.True(t, childAcl.AclState().LegalOwner().Equals(carolKeys.SignKey.GetPublic()))
}

func TestNestedSpaces_LegalOwnerUpdateRejections(t *testing.T) {
	aliceKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	bobKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	strangerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)

	t.Run("author must be the final owner", func(t *testing.T) {
		_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())
		proof := makeOwnershipProof(t, aliceKeys, bobKeys.SignKey.GetPublic())
		update := buildAclRecordSignedBy(t, childAcl.Head().Id, strangerKeys, legalOwnerUpdateContent(proof))
		require.ErrorIs(t, childAcl.AddRawRecord(update), ErrInsufficientPermissions)
	})

	t.Run("chain must start at the stored owner", func(t *testing.T) {
		_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())
		proof := makeOwnershipProof(t, strangerKeys, bobKeys.SignKey.GetPublic())
		update := buildAclRecordSignedBy(t, childAcl.Head().Id, bobKeys, legalOwnerUpdateContent(proof))
		require.ErrorIs(t, childAcl.AddRawRecord(update), ErrInvalidLegalOwnerProof)
	})

	t.Run("empty proof list", func(t *testing.T) {
		_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())
		update := buildAclRecordSignedBy(t, childAcl.Head().Id, aliceKeys, legalOwnerUpdateContent())
		require.ErrorIs(t, childAcl.AddRawRecord(update), ErrInvalidLegalOwnerProof)
	})

	t.Run("not a child space", func(t *testing.T) {
		a := NewAclExecutor("spaceId")
		require.NoError(t, a.Execute("a.init::a"))
		ownerAcl := a.ActualAccounts()["a"].Acl
		proof := makeOwnershipProof(t, aliceKeys, bobKeys.SignKey.GetPublic())
		update := buildAclRecordSignedBy(t, ownerAcl.Head().Id, bobKeys, legalOwnerUpdateContent(proof))
		require.ErrorIs(t, ownerAcl.AddRawRecord(update), ErrNotChildSpace)
	})

	t.Run("consumed proof cannot be replayed after an ownership cycle", func(t *testing.T) {
		_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())
		aliceToBob := makeOwnershipProof(t, aliceKeys, bobKeys.SignKey.GetPublic())
		bobToAlice := makeOwnershipProof(t, bobKeys, aliceKeys.SignKey.GetPublic())

		update := buildAclRecordSignedBy(t, childAcl.Head().Id, bobKeys, legalOwnerUpdateContent(aliceToBob))
		require.NoError(t, childAcl.AddRawRecord(update))
		update = buildAclRecordSignedBy(t, childAcl.Head().Id, aliceKeys, legalOwnerUpdateContent(bobToAlice))
		require.NoError(t, childAcl.AddRawRecord(update))
		require.True(t, childAcl.AclState().LegalOwner().Equals(aliceKeys.SignKey.GetPublic()))

		// bob replays the consumed alice->bob record to reclaim the child
		replay := buildAclRecordSignedBy(t, childAcl.Head().Id, bobKeys, legalOwnerUpdateContent(aliceToBob))
		require.ErrorIs(t, childAcl.AddRawRecord(replay), ErrInvalidLegalOwnerProof)
	})

	t.Run("duplicate proof within one update", func(t *testing.T) {
		_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())
		proof := makeOwnershipProof(t, aliceKeys, aliceKeys.SignKey.GetPublic())
		update := buildAclRecordSignedBy(t, childAcl.Head().Id, aliceKeys, legalOwnerUpdateContent(proof, proof))
		require.ErrorIs(t, childAcl.AddRawRecord(update), ErrInvalidLegalOwnerProof)
	})

	t.Run("proof with tampered signature", func(t *testing.T) {
		_, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())
		proof := makeOwnershipProof(t, aliceKeys, bobKeys.SignKey.GetPublic())
		var raw consensusproto.RawRecord
		require.NoError(t, raw.UnmarshalVT(proof))
		raw.Signature[0] ^= 0xff
		tampered, err := raw.MarshalVT()
		require.NoError(t, err)
		update := buildAclRecordSignedBy(t, childAcl.Head().Id, bobKeys, legalOwnerUpdateContent(tampered))
		require.ErrorIs(t, childAcl.AddRawRecord(update), ErrInvalidSignature)
	})
}

func TestNestedSpaces_StateCopyKeepsNestedFields(t *testing.T) {
	aliceKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	bobKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	childKeys, childAcl := newChildAcl(t, "parent.id", aliceKeys.SignKey.GetPublic())
	_ = childKeys

	proof := makeOwnershipProof(t, aliceKeys, bobKeys.SignKey.GetPublic())
	update := buildAclRecordSignedBy(t, childAcl.Head().Id, bobKeys, legalOwnerUpdateContent(proof))
	require.NoError(t, childAcl.AddRawRecord(update))

	cp := childAcl.AclState().Copy()
	require.Equal(t, "parent.id", cp.ParentSpaceId())
	require.True(t, cp.LegalOwner().Equals(bobKeys.SignKey.GetPublic()))
	// the replay guard survives the copy
	replay := buildAclRecordSignedBy(t, childAcl.Head().Id, bobKeys, legalOwnerUpdateContent(proof))
	rec, err := childAcl.RecordBuilder().UnmarshallWithId(replay)
	require.NoError(t, err)
	require.ErrorIs(t, cp.ApplyRecord(rec), ErrInvalidLegalOwnerProof)
}
