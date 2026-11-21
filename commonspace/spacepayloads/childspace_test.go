package spacepayloads

import (
	mrand "math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

func newChildCreatePayload(t *testing.T, parentSpaceId string, replicationKey uint64) SpaceCreatePayload {
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
		ReplicationKey: replicationKey,
		SpacePayload:   randBytes(12),
		MasterKey:      master,
		ReadKey:        readKey,
		MetadataKey:    metaKey,
		Metadata:       randBytes(6),
		ParentSpaceId:  parentSpaceId,
	}
}

func TestStoragePayloadForChildSpace(t *testing.T) {
	parentRepKey := mrand.Uint64()
	parentId := NewSpaceId("parentcid", parentRepKey)

	t.Run("the v1 header names the parent", func(t *testing.T) {
		out, err := StoragePayloadForSpaceCreateV1(newChildCreatePayload(t, parentId, parentRepKey))
		require.NoError(t, err)
		require.NoError(t, ValidateSpaceStorageCreatePayload(out))

		header := decodeSpaceHeader(t, out)
		require.Equal(t, parentId, header.GetParentSpaceId())
		require.Equal(t, spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion1, header.GetVersion())
		// the child takes the replication key it is given, here the parent's
		require.True(t, strings.HasSuffix(out.SpaceHeaderWithId.Id, "."+strconv.FormatUint(parentRepKey, 36)))
	})

	t.Run("without a parent the header names none", func(t *testing.T) {
		out, err := StoragePayloadForSpaceCreateV1(newChildCreatePayload(t, "", parentRepKey))
		require.NoError(t, err)
		require.NoError(t, ValidateSpaceStorageCreatePayload(out))
		require.Empty(t, decodeSpaceHeader(t, out).GetParentSpaceId())
	})

	t.Run("the non-v1 builder refuses a parent", func(t *testing.T) {
		_, err := StoragePayloadForSpaceCreate(newChildCreatePayload(t, parentId, parentRepKey))
		require.ErrorIs(t, err, ErrChildNeedsHeaderV1)
	})

	t.Run("the parent is covered by the header signature", func(t *testing.T) {
		out, err := StoragePayloadForSpaceCreateV1(newChildCreatePayload(t, parentId, parentRepKey))
		require.NoError(t, err)

		// another parent in the signed body, under the original signature
		var raw spacesyncproto.RawSpaceHeader
		require.NoError(t, raw.UnmarshalVT(out.SpaceHeaderWithId.RawHeader))
		var h spacesyncproto.SpaceHeader
		require.NoError(t, h.UnmarshalVT(raw.SpaceHeader))
		h.ParentSpaceId = NewSpaceId("othercid", parentRepKey)
		raw.SpaceHeader, err = h.MarshalVT()
		require.NoError(t, err)
		tamperedRaw, err := raw.MarshalVT()
		require.NoError(t, err)

		// the id matches the new bytes, so the check reaches the signature
		newCid, err := cidutil.NewCidFromBytes(tamperedRaw)
		require.NoError(t, err)
		tampered := &spacesyncproto.RawSpaceHeaderWithId{RawHeader: tamperedRaw, Id: NewSpaceId(newCid, h.ReplicationKey)}
		_, err = ValidateSpaceHeader(tampered, nil, out.AclWithId.Payload, out.SpaceSettingsWithId.RawChange)
		require.ErrorIs(t, err, spacestorage.ErrIncorrectSpaceHeader)
	})
}
