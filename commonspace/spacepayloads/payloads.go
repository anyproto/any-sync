package spacepayloads

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

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

type SpaceCreatePayload struct {
	// SigningKey is the signing key of the owner
	SigningKey crypto.PrivKey
	// SpaceType is an arbitrary string
	SpaceType string
	// ReplicationKey is a key which is to be used to determine the node where the space should be held
	ReplicationKey uint64
	// SpacePayload is an arbitrary payload related to space type
	SpacePayload []byte // we probably should put onetooneinfo here
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
var ErrIncorrectOneToOnePayload = errors.New("incorrect onetoone payload")

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
	marshalled, err := header.MarshalVT()
	if err != nil {
		return
	}
	signature, err := payload.SigningKey.Sign(marshalled)
	if err != nil {
		return
	}
	rawHeader := &spacesyncproto.RawSpaceHeader{SpaceHeader: marshalled, Signature: signature}
	marshalled, err = rawHeader.MarshalVT()
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
	aclBuilder := list.NewAclRecordBuilder("", keyStorage, nil, recordverifier.NewValidateFull())
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

func StoragePayloadForSpaceCreateV1(payload SpaceCreatePayload) (storagePayload spacestorage.SpaceStorageCreatePayload, err error) {
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
		Version:            spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion1,
	}

	// building acl root
	keyStorage := crypto.NewKeyStorage()
	aclBuilder := list.NewAclRecordBuilder("", keyStorage, nil, recordverifier.NewValidateFull())
	aclRoot, err := aclBuilder.BuildRoot(list.RootContent{
		PrivKey:   payload.SigningKey,
		MasterKey: payload.MasterKey,
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
		Seed:       spaceSettingsSeed,
		ChangeType: SpaceReserved,
		Timestamp:  time.Now().Unix(),
	})
	if err != nil {
		return
	}

	// build header
	header.AclPayload = aclRoot.Payload
	header.SettingPayload = settingsRoot.RawChange

	marshalled, err := header.MarshalVT()
	if err != nil {
		return
	}
	signature, err := payload.SigningKey.Sign(marshalled)
	if err != nil {
		return
	}
	rawHeader := &spacesyncproto.RawSpaceHeader{SpaceHeader: marshalled, Signature: signature}
	marshalled, err = rawHeader.MarshalVT()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalled)
	spaceId := NewSpaceId(id, payload.ReplicationKey)
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marshalled,
		Id:        spaceId,
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
	marshalled, err := header.MarshalVT()
	if err != nil {
		return
	}
	signature, err := payload.SigningKey.Sign(marshalled)
	if err != nil {
		return
	}
	rawHeader := &spacesyncproto.RawSpaceHeader{SpaceHeader: marshalled, Signature: signature}
	marshalled, err = rawHeader.MarshalVT()
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
	aclBuilder := list.NewAclRecordBuilder("", keyStorage, nil, recordverifier.NewValidateFull())
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

func makeOneToOneInfo(sharedSk crypto.PrivKey, aPk, bPk crypto.PubKey) (oneToOneInfo aclrecordproto.AclOneToOneInfo, err error) {
	writers := make([][]byte, 2)

	writers[0], err = aPk.Marshall()
	if err != nil {
		err = fmt.Errorf("makeOneToOneInfo: failed to Marshal account pub key: %w", err)
		return
	}
	writers[1], err = bPk.Marshall()
	if err != nil {
		err = fmt.Errorf("makeOneToOneInfo: failed to Marshal bPk: %w", err)
		return
	}

	// sort for idempotent spaceid creation
	sort.Slice(writers, func(i, j int) bool {
		return bytes.Compare(writers[i], writers[j]) < 0
	})

	sharedPkBytes, err := sharedSk.GetPublic().Marshall()
	if err != nil {
		err = fmt.Errorf("makeOneToOneInfo: failed to Marshal sharedPk: %w", err)
		return
	}
	oneToOneInfo = aclrecordproto.AclOneToOneInfo{
		Owner:   sharedPkBytes,
		Writers: writers,
	}
	return
}

func StoragePayloadForOneToOneSpace(aSk crypto.PrivKey, bPk crypto.PubKey) (storagePayload spacestorage.SpaceStorageCreatePayload, err error) {
	sharedSk, err := crypto.GenerateSharedKey(aSk, bPk, crypto.AnysyncOneToOneSpacePath)
	if err != nil {
		return
	}

	oneToOneInfo, err := makeOneToOneInfo(sharedSk, aSk.GetPublic(), bPk)
	if err != nil {
		return
	}

	oneToOneInfoBytes, err := oneToOneInfo.MarshalVT()
	if err != nil {
		err = fmt.Errorf("CreateOneToOneKeys: failed to Marshal oneToOneInfo: %w", err)
		return
	}

	// marshalling keys
	identity, err := sharedSk.GetPublic().Marshall()
	if err != nil {
		return
	}
	pubKey, err := sharedSk.GetPublic().Raw()
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

	// building acl root
	keyStorage := crypto.NewKeyStorage()
	aclBuilder := list.NewAclRecordBuilder("", keyStorage, nil, recordverifier.NewValidateFull())
	aclRoot, err := aclBuilder.BuildOneToOneRoot(list.RootContent{
		PrivKey:   sharedSk,
		MasterKey: sharedSk,
	}, &oneToOneInfo)
	if err != nil {
		return
	}

	// building settings
	builder := objecttree.NewChangeBuilder(keyStorage, nil)
	_, settingsRoot, err := builder.BuildRoot(objecttree.InitialContent{
		AclHeadId:  aclRoot.Id,
		PrivKey:    sharedSk,
		ChangeType: SpaceReserved,
	})
	if err != nil {
		return
	}

	// preparing header and space id
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		SpaceType:          "anytype.onetoone",
		SpaceHeaderPayload: oneToOneInfoBytes,
		ReplicationKey:     repKey,
		SettingPayload:     settingsRoot.RawChange,
		Version:            spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion1,
		AclPayload:         aclRoot.Payload,
	}
	marshalled, err := header.MarshalVT()
	if err != nil {
		return
	}
	signature, err := sharedSk.Sign(marshalled)
	if err != nil {
		return
	}
	rawHeader := &spacesyncproto.RawSpaceHeader{SpaceHeader: marshalled, Signature: signature}
	marshalled, err = rawHeader.MarshalVT()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalled)
	spaceId := NewSpaceId(id, repKey)
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marshalled,
		Id:        spaceId,
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
	needCheckSpaceId, err := ValidateSpaceHeader(payload.SpaceHeaderWithId, nil, payload.AclWithId.Payload, payload.SpaceSettingsWithId.RawChange)
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
	if needCheckSpaceId {
		if aclSpaceId != payload.SpaceHeaderWithId.Id || aclSpaceId != settingsSpaceId {
			err = spacestorage.ErrIncorrectSpaceHeader
			return
		}
	}
	if aclHeadId != payload.AclWithId.Id {
		err = spacestorage.ErrIncorrectSpaceHeader
		return
	}
	return
}

func ValidateSpaceHeader(rawHeaderWithId *spacesyncproto.RawSpaceHeaderWithId, identity crypto.PubKey, aclPayload []byte, settingsPayload []byte) (needCheckSpaceId bool, err error) {
	if rawHeaderWithId == nil {
		return false, spacestorage.ErrIncorrectSpaceHeader
	}
	sepIdx := strings.Index(rawHeaderWithId.Id, ".")
	if sepIdx == -1 {
		return false, spacestorage.ErrIncorrectSpaceHeader
	}
	if !cidutil.VerifyCid(rawHeaderWithId.RawHeader, rawHeaderWithId.Id[:sepIdx]) {
		return false, objecttree.ErrIncorrectCid
	}
	var rawSpaceHeader spacesyncproto.RawSpaceHeader
	err = rawSpaceHeader.UnmarshalVT(rawHeaderWithId.RawHeader)
	if err != nil {
		return
	}
	var header spacesyncproto.SpaceHeader
	err = header.UnmarshalVT(rawSpaceHeader.SpaceHeader)
	if err != nil {
		return
	}
	payloadIdentity, err := crypto.UnmarshalEd25519PublicKeyProto(header.Identity)
	if err != nil {
		return
	}
	res, err := payloadIdentity.Verify(rawSpaceHeader.SpaceHeader, rawSpaceHeader.Signature)
	if err != nil || !res {
		return false, spacestorage.ErrIncorrectSpaceHeader
	}
	if rawHeaderWithId.Id[sepIdx+1:] != strconv.FormatUint(header.ReplicationKey, 36) {
		return false, spacestorage.ErrIncorrectSpaceHeader
	}
	isV1 := header.Version == spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion1
	needCheckSpaceId = !isV1

	if isV1 {
		if aclPayload != nil && !bytes.Equal(aclPayload, header.AclPayload) {
			return false, spacestorage.ErrIncorrectSpaceHeader
		}
		if settingsPayload != nil && !bytes.Equal(settingsPayload, header.SettingPayload) {
			return false, spacestorage.ErrIncorrectSpaceHeader
		}
	}

	if header.SpaceType == "anytype.onetoone" {
		var oneToOneInfo aclrecordproto.AclOneToOneInfo
		err = oneToOneInfo.UnmarshalVT(header.SpaceHeaderPayload)
		if err != nil {
			return false, ErrIncorrectOneToOnePayload
		}
	} else if identity != nil && !payloadIdentity.Equals(identity) {
		return false, ErrIncorrectIdentity
	}

	return
}

func validateCreateSpaceAclPayload(rawWithId *consensusproto.RawRecordWithId) (spaceId string, err error) {
	if !cidutil.VerifyCid(rawWithId.Payload, rawWithId.Id) {
		err = objecttree.ErrIncorrectCid
		return
	}
	var rawAcl consensusproto.RawRecord
	err = rawAcl.UnmarshalVT(rawWithId.Payload)
	if err != nil {
		return
	}
	var aclRoot aclrecordproto.AclRoot
	err = aclRoot.UnmarshalVT(rawAcl.Payload)
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
	err = raw.UnmarshalVT(rawWithId.RawChange)
	if err != nil {
		return
	}
	var rootChange treechangeproto.RootChange
	err = rootChange.UnmarshalVT(raw.Payload)
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
	return id + "." + strconv.FormatUint(repKey, 36)
}
