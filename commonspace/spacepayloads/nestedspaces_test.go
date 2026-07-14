package spacepayloads

import (
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func newChildSpaceCreatePayload(t *testing.T, parentSpaceId string, legalOwner crypto.PubKey) SpaceCreatePayload {
	acc, err := accountdata.NewRandom()
	require.NoError(t, err)
	master, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	metaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	readKey, _ := crypto.NewRandomAES()
	return SpaceCreatePayload{
		SigningKey:     acc.SignKey,
		SpaceType:      "test.space",
		ReplicationKey: mrand.Uint64(),
		MasterKey:      master,
		ReadKey:        readKey,
		MetadataKey:    metaKey,
		Metadata:        randBytes(6),
		ParentSpaceId:   parentSpaceId,
		LegalOwner:      legalOwner,
		ParentAclRootId: "parent-acl-root-id",
	}
}

func TestStoragePayloadForChildSpaceCreateV1(t *testing.T) {
	parentOwner, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)

	t.Run("child payload builds and validates", func(t *testing.T) {
		pl := newChildSpaceCreatePayload(t, "parent.id", parentOwner.GetPublic())
		out, err := StoragePayloadForSpaceCreateV1(pl)
		require.NoError(t, err)
		require.NoError(t, ValidateSpaceStorageCreatePayload(out))

		// header carries the parent link
		var rawHeader spacesyncproto.RawSpaceHeader
		require.NoError(t, rawHeader.UnmarshalVT(out.SpaceHeaderWithId.RawHeader))
		var header spacesyncproto.SpaceHeader
		require.NoError(t, header.UnmarshalVT(rawHeader.SpaceHeader))
		require.Equal(t, "parent.id", header.ParentSpaceId)

		// acl root mirrors it and pins the legal owner
		var rawAcl consensusproto.RawRecord
		require.NoError(t, rawAcl.UnmarshalVT(out.AclWithId.Payload))
		var aclRoot aclrecordproto.AclRoot
		require.NoError(t, aclRoot.UnmarshalVT(rawAcl.Payload))
		require.Equal(t, "parent.id", aclRoot.ParentSpaceId)
		legalOwner, err := crypto.UnmarshalEd25519PublicKeyProto(aclRoot.LegalOwner)
		require.NoError(t, err)
		require.True(t, legalOwner.Equals(parentOwner.GetPublic()))
	})

	t.Run("parent link requires legal owner", func(t *testing.T) {
		pl := newChildSpaceCreatePayload(t, "parent.id", nil)
		_, err := StoragePayloadForSpaceCreateV1(pl)
		require.ErrorIs(t, err, list.ErrIncorrectRoot)
	})

	t.Run("v0 create rejects a parent link", func(t *testing.T) {
		pl := newChildSpaceCreatePayload(t, "parent.id", parentOwner.GetPublic())
		_, err := StoragePayloadForSpaceCreate(pl)
		require.ErrorIs(t, err, ErrIncorrectParentLink)
	})

	t.Run("validateParentLink rejects a non-V1 header carrying a parent link", func(t *testing.T) {
		legalOwner, err := parentOwner.GetPublic().Marshall()
		require.NoError(t, err)
		aclRoot := &aclrecordproto.AclRoot{
			ParentSpaceId:   "parent.id",
			LegalOwner:      legalOwner,
			ParentAclRootId: "parent-acl-root-id",
		}
		mkHeader := func(version spacesyncproto.SpaceHeaderVersion) *spacesyncproto.RawSpaceHeaderWithId {
			h := &spacesyncproto.SpaceHeader{ParentSpaceId: "parent.id", Version: version}
			hb, err := h.MarshalVT()
			require.NoError(t, err)
			raw, err := (&spacesyncproto.RawSpaceHeader{SpaceHeader: hb}).MarshalVT()
			require.NoError(t, err)
			return &spacesyncproto.RawSpaceHeaderWithId{RawHeader: raw}
		}
		// V0 child header is rejected...
		require.ErrorIs(t, validateParentLink(mkHeader(spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion0), aclRoot), ErrIncorrectParentLink)
		// ...V1 with the matching parent link passes
		require.NoError(t, validateParentLink(mkHeader(spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion1), aclRoot))
	})
}
