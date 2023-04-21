package commonspace

import (
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	SpaceReserved = "any-sync.space"
)

func storagePayloadForSpaceCreate(payload SpaceCreatePayload) (storagePayload spacestorage.SpaceStorageCreatePayload, err error) {
	// marshalling keys
	identity, err := payload.SigningKey.GetPublic().Marshall()
	if err != nil {
		return
	}

	// preparing header and space id
	spaceHeaderSeed := make([]byte, 32)
	_, err = rand.Read(spaceHeaderSeed)
	if err != nil {
		return
	}
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		Timestamp:          time.Now().Unix(),
		SpaceType:          payload.SpaceType,
		SpaceHeaderPayload: payload.SpacePayload,
		ReplicationKey:     payload.ReplicationKey,
		Seed:               spaceHeaderSeed,
	}
	marshalled, err := header.Marshal()
	if err != nil {
		return
	}
	signature, err := payload.SigningKey.Sign(marshalled)
	if err != nil {
		return
	}
	rawHeader := &spacesyncproto.RawSpaceHeader{SpaceHeader: marshalled, Signature: signature}
	marshalled, err = rawHeader.Marshal()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalled)
	spaceId := NewSpaceId(id, payload.ReplicationKey)
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marshalled,
		Id:        spaceId,
	}
	readKey, err := payload.SigningKey.GetPublic().Encrypt(payload.ReadKey)
	if err != nil {
		return
	}

	// building acl root
	keyStorage := crypto.NewKeyStorage()
	aclBuilder := list.NewAclRecordBuilder("", keyStorage)
	aclRoot, err := aclBuilder.BuildRoot(list.RootContent{
		PrivKey:          payload.SigningKey,
		MasterKey:        payload.MasterKey,
		SpaceId:          spaceId,
		EncryptedReadKey: readKey,
	})
	if err != nil {
		return
	}

	// building settings
	builder := objecttree.NewChangeBuilder(keyStorage, nil)
	spaceSettingsSeed := make([]byte, 32)
	_, err = rand.Read(spaceSettingsSeed)
	if err != nil {
		return
	}
	_, settingsRoot, err := builder.BuildRoot(objecttree.InitialContent{
		AclHeadId:  aclRoot.Id,
		PrivKey:    payload.SigningKey,
		SpaceId:    spaceId,
		Seed:       spaceSettingsSeed,
		ChangeType: SpaceReserved,
		Timestamp:  time.Now().Unix(),
	})
	if err != nil {
		return
	}

	// creating storage
	storagePayload = spacestorage.SpaceStorageCreatePayload{
		AclWithId:           aclRoot,
		SpaceHeaderWithId:   rawHeaderWithId,
		SpaceSettingsWithId: settingsRoot,
	}
	return
}

func storagePayloadForSpaceDerive(payload SpaceDerivePayload) (storagePayload spacestorage.SpaceStorageCreatePayload, err error) {
	// marshalling keys
	identity, err := payload.SigningKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	pubKey, err := payload.SigningKey.GetPublic().Raw()
	if err != nil {
		return
	}

	// preparing replication key
	hasher := fnv.New64()
	_, err = hasher.Write(pubKey)
	if err != nil {
		return
	}
	repKey := hasher.Sum64()

	// preparing header and space id
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		SpaceType:          payload.SpaceType,
		SpaceHeaderPayload: payload.SpacePayload,
		ReplicationKey:     repKey,
	}
	marshalled, err := header.Marshal()
	if err != nil {
		return
	}
	signature, err := payload.SigningKey.Sign(marshalled)
	if err != nil {
		return
	}
	rawHeader := &spacesyncproto.RawSpaceHeader{SpaceHeader: marshalled, Signature: signature}
	marshalled, err = rawHeader.Marshal()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalled)
	spaceId := NewSpaceId(id, repKey)
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marshalled,
		Id:        spaceId,
	}

	// building acl root
	keyStorage := crypto.NewKeyStorage()
	aclBuilder := list.NewAclRecordBuilder("", keyStorage)
	aclRoot, err := aclBuilder.BuildRoot(list.RootContent{
		PrivKey:   payload.SigningKey,
		MasterKey: payload.MasterKey,
		SpaceId:   spaceId,
	})
	if err != nil {
		return
	}

	// building settings
	builder := objecttree.NewChangeBuilder(keyStorage, nil)
	_, settingsRoot, err := builder.BuildRoot(objecttree.InitialContent{
		AclHeadId:  aclRoot.Id,
		PrivKey:    payload.SigningKey,
		SpaceId:    spaceId,
		ChangeType: SpaceReserved,
	})
	if err != nil {
		return
	}

	// creating storage
	storagePayload = spacestorage.SpaceStorageCreatePayload{
		AclWithId:           aclRoot,
		SpaceHeaderWithId:   rawHeaderWithId,
		SpaceSettingsWithId: settingsRoot,
	}
	return
}

func validateSpaceStorageCreatePayload(payload spacestorage.SpaceStorageCreatePayload) (err error) {
	err = validateCreateSpaceHeaderPayload(payload.SpaceHeaderWithId)
	if err != nil {
		return
	}
	aclSpaceId, err := validateCreateSpaceAclPayload(payload.AclWithId)
	if err != nil {
		return
	}
	aclHeadId, settingsSpaceId, err := validateCreateSpaceSettingsPayload(payload.SpaceSettingsWithId)
	if err != nil {
		return
	}
	if aclSpaceId != payload.SpaceHeaderWithId.Id || aclSpaceId != settingsSpaceId {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	if aclHeadId != payload.AclWithId.Id {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	return
}

func validateCreateSpaceHeaderPayload(rawHeaderWithId *spacesyncproto.RawSpaceHeaderWithId) (err error) {
	var rawSpaceHeader spacesyncproto.RawSpaceHeader
	err = proto.Unmarshal(rawHeaderWithId.RawHeader, &rawSpaceHeader)
	if err != nil {
		return
	}
	var header spacesyncproto.SpaceHeader
	err = proto.Unmarshal(rawSpaceHeader.SpaceHeader, &header)
	if err != nil {
		return
	}
	split := strings.Split(rawHeaderWithId.Id, ".")
	if len(split) != 2 {
		return spacestorage.ErrIncorrectSpaceHeader
	}
	if !cidutil.VerifyCid(rawHeaderWithId.RawHeader, split[0]) {
		err = objecttree.ErrIncorrectCid
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(header.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(rawSpaceHeader.SpaceHeader, rawSpaceHeader.Signature)
	if err != nil || !res {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	id, err := cidutil.NewCidFromBytes(rawHeaderWithId.RawHeader)
	if err != nil {
		return
	}
	requiredSpaceId := fmt.Sprintf("%s.%s", id, strconv.FormatUint(header.ReplicationKey, 36))
	if requiredSpaceId != rawHeaderWithId.Id {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}

	return
}

func validateCreateSpaceAclPayload(rawWithId *aclrecordproto.RawAclRecordWithId) (spaceId string, err error) {
	if !cidutil.VerifyCid(rawWithId.Payload, rawWithId.Id) {
		err = objecttree.ErrIncorrectCid
		return
	}
	var rawAcl aclrecordproto.RawAclRecord
	err = proto.Unmarshal(rawWithId.Payload, &rawAcl)
	if err != nil {
		return
	}
	var aclRoot aclrecordproto.AclRoot
	err = proto.Unmarshal(rawAcl.Payload, &aclRoot)
	if err != nil {
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(aclRoot.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(rawAcl.Payload, rawAcl.Signature)
	if err != nil || !res {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	masterKey, err := crypto.UnmarshalEd25519PublicKeyProto(aclRoot.MasterKey)
	if err != nil {
		return
	}
	rawIdentity, err := payloadIdentity.Raw()
	if err != nil {
		return
	}
	res, err = masterKey.Verify(rawIdentity, aclRoot.IdentitySignature)
	if err != nil || !res {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	spaceId = aclRoot.SpaceId

	return
}

func validateCreateSpaceSettingsPayload(rawWithId *treechangeproto.RawTreeChangeWithId) (aclHeadId string, spaceId string, err error) {
	var raw treechangeproto.RawTreeChange
	err = proto.Unmarshal(rawWithId.RawChange, &raw)
	if err != nil {
		return
	}
	var rootChange treechangeproto.RootChange
	err = proto.Unmarshal(raw.Payload, &rootChange)
	if err != nil {
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(rootChange.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(raw.Payload, raw.Signature)
	if err != nil || !res {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	id, err := cidutil.NewCidFromBytes(rawWithId.RawChange)
	if id != rawWithId.Id {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	spaceId = rootChange.SpaceId
	aclHeadId = rootChange.AclHeadId

	return
}

// ValidateSpaceHeader Used in coordinator
func ValidateSpaceHeader(spaceId string, header []byte, identity crypto.PubKey) (err error) {
	split := strings.Split(spaceId, ".")
	if len(split) != 2 {
		return spacestorage.ErrIncorrectSpaceHeader
	}
	if !cidutil.VerifyCid(header, split[0]) {
		err = objecttree.ErrIncorrectCid
		return
	}
	raw := &spacesyncproto.RawSpaceHeader{}
	err = proto.Unmarshal(header, raw)
	if err != nil {
		return
	}
	payload := &spacesyncproto.SpaceHeader{}
	err = proto.Unmarshal(raw.SpaceHeader, payload)
	if err != nil {
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(payload.Identity)
	if err != nil {
		return
	}
	if identity != nil && !payloadIdentity.Equals(identity) {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	res, err := identity.Verify(raw.SpaceHeader, raw.Signature)
	if err != nil || !res {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	return
}
