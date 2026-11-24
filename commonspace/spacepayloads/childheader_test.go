package spacepayloads

import (
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

// copyHeader is the header as another peer receives it: the raw bytes and the id only
func copyHeader(h *spacesyncproto.RawSpaceHeaderWithId) *spacesyncproto.RawSpaceHeaderWithId {
	return &spacesyncproto.RawSpaceHeaderWithId{RawHeader: append([]byte(nil), h.RawHeader...), Id: h.Id}
}

func parentOfRawHeader(t *testing.T, h *spacesyncproto.RawSpaceHeaderWithId) string {
	var raw spacesyncproto.RawSpaceHeader
	require.NoError(t, raw.UnmarshalVT(h.RawHeader))
	var header spacesyncproto.SpaceHeader
	require.NoError(t, header.UnmarshalVT(raw.SpaceHeader))
	return header.ParentSpaceId
}

// resignedHeader rewrites the signed body of a header, signs it with key and gives it the id of the new
// bytes
func resignedHeader(t *testing.T, h *spacesyncproto.RawSpaceHeaderWithId, key crypto.PrivKey, change func(header *spacesyncproto.SpaceHeader)) *spacesyncproto.RawSpaceHeaderWithId {
	var raw spacesyncproto.RawSpaceHeader
	require.NoError(t, raw.UnmarshalVT(h.RawHeader))
	var header spacesyncproto.SpaceHeader
	require.NoError(t, header.UnmarshalVT(raw.SpaceHeader))
	change(&header)
	body, err := header.MarshalVT()
	require.NoError(t, err)
	signature, err := key.Sign(body)
	require.NoError(t, err)
	rawHeader, err := (&spacesyncproto.RawSpaceHeader{SpaceHeader: body, Signature: signature}).MarshalVT()
	require.NoError(t, err)
	cid, err := cidutil.NewCidFromBytes(rawHeader)
	require.NoError(t, err)
	return &spacesyncproto.RawSpaceHeaderWithId{RawHeader: rawHeader, Id: NewSpaceId(cid, header.ReplicationKey)}
}

func TestChildHeader_ValidatesFromRawBytes(t *testing.T) {
	repKey := mrand.Uint64()
	parentId := NewSpaceId("parentcid", repKey)
	payload := newChildCreatePayload(t, parentId, repKey)
	out, err := StoragePayloadForSpaceCreateV1(payload)
	require.NoError(t, err)
	creator := payload.SigningKey.GetPublic()

	t.Run("the header alone", func(t *testing.T) {
		header := copyHeader(out.SpaceHeaderWithId)
		_, err := ValidateSpaceHeader(header, creator, nil, nil)
		require.NoError(t, err)
		require.Equal(t, parentId, parentOfRawHeader(t, header))
	})

	t.Run("with the acl root and settings it binds", func(t *testing.T) {
		header := copyHeader(out.SpaceHeaderWithId)
		acl := &consensusproto.RawRecordWithId{Payload: append([]byte(nil), out.AclWithId.Payload...), Id: out.AclWithId.Id}
		settings := &treechangeproto.RawTreeChangeWithId{
			RawChange: append([]byte(nil), out.SpaceSettingsWithId.RawChange...),
			Id:        out.SpaceSettingsWithId.Id,
		}
		_, err := ValidateSpaceHeader(header, creator, acl.Payload, settings.RawChange)
		require.NoError(t, err)
		require.NoError(t, ValidateSpaceStorageCreatePayload(spacestorage.SpaceStorageCreatePayload{
			AclWithId:           acl,
			SpaceHeaderWithId:   header,
			SpaceSettingsWithId: settings,
		}))
	})

	t.Run("signed by another account than the creator", func(t *testing.T) {
		other, err := accountdata.NewRandom()
		require.NoError(t, err)
		_, err = ValidateSpaceHeader(copyHeader(out.SpaceHeaderWithId), other.SignKey.GetPublic(), nil, nil)
		require.ErrorIs(t, err, ErrIncorrectIdentity)
	})

	t.Run("the acl root of another space", func(t *testing.T) {
		other, err := StoragePayloadForSpaceCreateV1(newChildCreatePayload(t, parentId, repKey))
		require.NoError(t, err)
		_, err = ValidateSpaceHeader(copyHeader(out.SpaceHeaderWithId), nil, other.AclWithId.Payload, nil)
		require.ErrorIs(t, err, spacestorage.ErrIncorrectSpaceHeader)
		_, err = ValidateSpaceHeader(copyHeader(out.SpaceHeaderWithId), nil, nil, other.SpaceSettingsWithId.RawChange)
		require.ErrorIs(t, err, spacestorage.ErrIncorrectSpaceHeader)
	})
}

// The parent link is part of the space id: it cannot change while the id stays the same.
func TestChildHeader_ParentIsBoundIntoTheId(t *testing.T) {
	repKey := mrand.Uint64()
	parentId := NewSpaceId("parentcid", repKey)
	payload := newChildCreatePayload(t, parentId, repKey)
	out, err := StoragePayloadForSpaceCreateV1(payload)
	require.NoError(t, err)

	for name, parent := range map[string]string{
		"another parent": NewSpaceId("othercid", repKey),
		"no parent":      "",
	} {
		t.Run(name, func(t *testing.T) {
			// re-signed by the creator: a valid header, but of another space
			changed := resignedHeader(t, out.SpaceHeaderWithId, payload.SigningKey, func(h *spacesyncproto.SpaceHeader) {
				h.ParentSpaceId = parent
			})
			_, err := ValidateSpaceHeader(changed, payload.SigningKey.GetPublic(), nil, nil)
			require.NoError(t, err)
			require.NotEqual(t, out.SpaceHeaderWithId.Id, changed.Id)

			// under the id of the child it is refused
			_, err = ValidateSpaceHeader(&spacesyncproto.RawSpaceHeaderWithId{
				RawHeader: changed.RawHeader,
				Id:        out.SpaceHeaderWithId.Id,
			}, nil, nil, nil)
			require.ErrorIs(t, err, objecttree.ErrIncorrectCid)
		})
	}
}

// A child needs the V1 header, which binds its acl root into the signed header: a header that names a
// parent without the V1 version does not validate.
func TestChildHeader_RequiresV1OnValidation(t *testing.T) {
	repKey := mrand.Uint64()
	payload := newChildCreatePayload(t, NewSpaceId("parentcid", repKey), repKey)
	out, err := StoragePayloadForSpaceCreateV1(payload)
	require.NoError(t, err)

	// signed by its creator, as a client that skips the v1 builder writes it
	v0 := resignedHeader(t, out.SpaceHeaderWithId, payload.SigningKey, func(h *spacesyncproto.SpaceHeader) {
		h.Version = spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion0
		h.AclPayload = nil
		h.SettingPayload = nil
	})
	require.NotEmpty(t, parentOfRawHeader(t, v0))

	_, err = ValidateSpaceHeader(v0, payload.SigningKey.GetPublic(), nil, nil)
	require.ErrorIs(t, err, ErrChildNeedsHeaderV1)
	err = ValidateSpaceStorageCreatePayload(spacestorage.SpaceStorageCreatePayload{
		AclWithId:           out.AclWithId,
		SpaceHeaderWithId:   v0,
		SpaceSettingsWithId: out.SpaceSettingsWithId,
	})
	require.ErrorIs(t, err, ErrChildNeedsHeaderV1)

	// the same header without the parent is a top-level space of the older header
	topLevel := resignedHeader(t, v0, payload.SigningKey, func(h *spacesyncproto.SpaceHeader) {
		h.ParentSpaceId = ""
	})
	_, err = ValidateSpaceHeader(topLevel, payload.SigningKey.GetPublic(), nil, nil)
	require.NoError(t, err)
}

// Only a child names a parent: every builder of a top-level space leaves the field empty.
func TestTopLevelHeader_NamesNoParent(t *testing.T) {
	acc, err := accountdata.NewRandom()
	require.NoError(t, err)
	master, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	other, err := accountdata.NewRandom()
	require.NoError(t, err)
	derive := SpaceDerivePayload{SigningKey: acc.SignKey, MasterKey: master, SpaceType: "derived.space", SpacePayload: randBytes(10)}

	for name, build := range map[string]func() (spacestorage.SpaceStorageCreatePayload, error){
		"create": func() (spacestorage.SpaceStorageCreatePayload, error) {
			return StoragePayloadForSpaceCreate(newChildCreatePayload(t, "", mrand.Uint64()))
		},
		"create v1": func() (spacestorage.SpaceStorageCreatePayload, error) {
			return StoragePayloadForSpaceCreateV1(newChildCreatePayload(t, "", mrand.Uint64()))
		},
		"derive":    func() (spacestorage.SpaceStorageCreatePayload, error) { return StoragePayloadForSpaceDerive(derive) },
		"derive v1": func() (spacestorage.SpaceStorageCreatePayload, error) { return StoragePayloadForSpaceDeriveV1(derive) },
		"one-to-one": func() (spacestorage.SpaceStorageCreatePayload, error) {
			return StoragePayloadForOneToOneSpace(acc.SignKey, other.SignKey.GetPublic())
		},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := build()
			require.NoError(t, err)
			require.NoError(t, ValidateSpaceStorageCreatePayload(out))
			require.Empty(t, parentOfRawHeader(t, out.SpaceHeaderWithId))
		})
	}
}

// withAclRoot is the storage payload of the space with another acl root: the settings name the root, and
// the creator signs the header that binds both.
func withAclRoot(t *testing.T, payload SpaceCreatePayload, root *consensusproto.RawRecordWithId) spacestorage.SpaceStorageCreatePayload {
	out, err := StoragePayloadForSpaceCreateV1(payload)
	require.NoError(t, err)
	_, settings, err := objecttree.NewChangeBuilder(crypto.NewKeyStorage(), nil).BuildRoot(objecttree.InitialContent{
		AclHeadId:  root.Id,
		PrivKey:    payload.SigningKey,
		Seed:       randBytes(32),
		ChangeType: SpaceReserved,
	})
	require.NoError(t, err)
	header := resignedHeader(t, out.SpaceHeaderWithId, payload.SigningKey, func(h *spacesyncproto.SpaceHeader) {
		h.AclPayload, h.SettingPayload = root.Payload, settings.RawChange
	})
	return spacestorage.SpaceStorageCreatePayload{AclWithId: root, SpaceHeaderWithId: header, SpaceSettingsWithId: settings}
}

// The creator of a child is its first owner: the acl root of a child is written by the account that
// signed its header, and it is not a one-to-one root. A top-level space takes any root.
func TestChildHeader_AclRootIsTheCreators(t *testing.T) {
	repKey := mrand.Uint64()
	child := newChildCreatePayload(t, NewSpaceId("parentcid", repKey), repKey)
	topLevel := newChildCreatePayload(t, "", repKey)
	outsider, err := accountdata.NewRandom()
	require.NoError(t, err)
	outsiderKey, err := outsider.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	builder := list.NewAclRecordBuilder("", crypto.NewKeyStorage(), nil, recordverifier.NewValidateFull())
	rootOf := func(author crypto.PrivKey) *consensusproto.RawRecordWithId {
		root, err := builder.BuildRoot(list.RootContent{PrivKey: author, MasterKey: author})
		require.NoError(t, err)
		return root
	}
	// notOwn are the roots that are not the creator's own
	notOwn := func(creator crypto.PrivKey) map[string]*consensusproto.RawRecordWithId {
		// the creator builds the one-to-one space with the outsider alone
		oneToOne, err := StoragePayloadForOneToOneSpace(creator, outsider.SignKey.GetPublic())
		require.NoError(t, err)
		// a one-to-one root names the accounts of the space itself, whoever signs it
		creatorKey, err := creator.GetPublic().Marshall()
		require.NoError(t, err)
		named, err := builder.BuildOneToOneRoot(list.RootContent{PrivKey: creator, MasterKey: creator}, &aclrecordproto.AclOneToOneInfo{
			Owner:   outsiderKey,
			Writers: [][]byte{creatorKey, outsiderKey},
		})
		require.NoError(t, err)
		return map[string]*consensusproto.RawRecordWithId{
			"the root of another account":              rootOf(outsider.SignKey),
			"the root of a one-to-one space":           oneToOne.AclWithId,
			"a one-to-one root that the creator signs": named,
		}
	}

	t.Run("a child with the root of its creator", func(t *testing.T) {
		require.NoError(t, ValidateSpaceStorageCreatePayload(withAclRoot(t, child, rootOf(child.SigningKey))))
	})
	for name, root := range notOwn(child.SigningKey) {
		t.Run("a child with "+name, func(t *testing.T) {
			require.ErrorIs(t, ValidateSpaceStorageCreatePayload(withAclRoot(t, child, root)), ErrChildNeedsOwnAclRoot)
		})
	}
	for name, root := range notOwn(topLevel.SigningKey) {
		t.Run("a top-level space with "+name, func(t *testing.T) {
			require.NoError(t, ValidateSpaceStorageCreatePayload(withAclRoot(t, topLevel, root)))
		})
	}
	t.Run("a one-to-one space", func(t *testing.T) {
		oneToOne, err := StoragePayloadForOneToOneSpace(child.SigningKey, outsider.SignKey.GetPublic())
		require.NoError(t, err)
		require.NoError(t, ValidateSpaceStorageCreatePayload(oneToOne))
	})
}
