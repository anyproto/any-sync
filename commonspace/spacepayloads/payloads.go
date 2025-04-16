package spacepayloads

import (
	"crypto/rand"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/anyproto/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

type SpaceCreatePayload struct {
	// SigningKey is the signing key of the owner
	SigningKey crypto.PrivKey
	// SpaceType is an arbitrary string
	SpaceType string
	// ReplicationKey is a key which is to be used to determine the node where the space should be held
	ReplicationKey uint64
	// SpacePayload is an arbitrary payload related to space type
	SpacePayload []byte
	// MasterKey is the master key of the owner
	MasterKey crypto.PrivKey
	// ReadKey is the first read key of space
	ReadKey crypto.SymKey
	// MetadataKey is the first metadata key of space
	MetadataKey crypto.PrivKey
	// Metadata is the metadata of the owner
	Metadata []byte
}

type SpaceDerivePayload struct {
	SigningKey   crypto.PrivKey
	MasterKey    crypto.PrivKey
	SpaceType    string
	SpacePayload []byte
}

const (
	SpaceReserved = "any-sync.space"
)

var ErrIncorrectIdentity = errors.New("incorrect identity")

func StoragePayloadForSpaceCreate(payload SpaceCreatePayload) (storagePayload spacestorage.SpaceStorageCreatePayload, err error) {
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

	// building acl root
	keyStorage := crypto.NewKeyStorage()
	aclBuilder := list.NewAclRecordBuilder("", keyStorage, nil, list.NoOpAcceptorVerifier{})
	aclRoot, err := aclBuilder.BuildRoot(list.RootContent{
		PrivKey:   payload.SigningKey,
		MasterKey: payload.MasterKey,
		SpaceId:   spaceId,
		Change: list.ReadKeyChangePayload{
			MetadataKey: payload.MetadataKey,
			ReadKey:     payload.ReadKey,
		},
		Metadata: payload.Metadata,
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

func StoragePayloadForSpaceDerive(payload SpaceDerivePayload) (storagePayload spacestorage.SpaceStorageCreatePayload, err error) {
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
	aclBuilder := list.NewAclRecordBuilder("", keyStorage, nil, list.NoOpAcceptorVerifier{})
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

func ValidateSpaceStorageCreatePayload(payload spacestorage.SpaceStorageCreatePayload) (err error) {
	err = ValidateSpaceHeader(payload.SpaceHeaderWithId, nil)
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

func ValidateSpaceHeader(rawHeaderWithId *spacesyncproto.RawSpaceHeaderWithId, identity crypto.PubKey) (err error) {
	if rawHeaderWithId == nil {
		return spacestorage.ErrIncorrectSpaceHeader
	}
	sepIdx := strings.Index(rawHeaderWithId.Id, ".")
	if sepIdx == -1 {
		return spacestorage.ErrIncorrectSpaceHeader
	}
	if !cidutil.VerifyCid(rawHeaderWithId.RawHeader, rawHeaderWithId.Id[:sepIdx]) {
		return objecttree.ErrIncorrectCid
	}
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
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(header.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(rawSpaceHeader.SpaceHeader, rawSpaceHeader.Signature)
	if err != nil || !res {
		return spacestorage.ErrIncorrectSpaceHeader
	}
	if rawHeaderWithId.Id[sepIdx+1:] != strconv.FormatUint(header.ReplicationKey, 36) {
		return spacestorage.ErrIncorrectSpaceHeader
	}
	if identity == nil {
		return
	}
	if !payloadIdentity.Equals(identity) {
		return ErrIncorrectIdentity
	}
	return
}

func validateCreateSpaceAclPayload(rawWithId *consensusproto.RawRecordWithId) (spaceId string, err error) {
	if !cidutil.VerifyCid(rawWithId.Payload, rawWithId.Id) {
		err = objecttree.ErrIncorrectCid
		return
	}
	var rawAcl consensusproto.RawRecord
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
	if !cidutil.VerifyCid(rawWithId.RawChange, rawWithId.Id) {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
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
	spaceId = rootChange.SpaceId
	aclHeadId = rootChange.AclHeadId

	return
}

func NewSpaceId(id string, repKey uint64) string {
	return strings.Join([]string{id, strconv.FormatUint(repKey, 36)}, ".")
}
