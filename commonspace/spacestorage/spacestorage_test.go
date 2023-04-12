package spacestorage

import (
	"crypto/rand"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rand2 "golang.org/x/exp/rand"
	"strconv"
	"testing"
	"time"
)

func TestSuccessHeaderPayloadForSpaceCreate(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	spaceHeaderSeed := make([]byte, 32)
	_, err = rand.Read(spaceHeaderSeed)
	require.NoError(t, err)
	spaceHeaderPayload := make([]byte, 32)
	_, err = rand.Read(spaceHeaderPayload)
	require.NoError(t, err)
	replicationKey := rand2.Uint64()
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		Timestamp:          time.Now().Unix(),
		SpaceType:          "SpaceType",
		ReplicationKey:     replicationKey,
		Seed:               spaceHeaderSeed,
		SpaceHeaderPayload: spaceHeaderPayload,
	}
	marhalled, err := header.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marhalled)
	require.NoError(t, err)
	rawHeader := &spacesyncproto.RawSpaceHeader{
		SpaceHeader: marhalled,
		Signature:   signature,
	}
	marhalledRawHeader, err := rawHeader.Marshal()
	require.NoError(t, err)
	id, err := cidutil.NewCidFromBytes(marhalled)
	require.NoError(t, err)
	spaceId := fmt.Sprintf("%s.%s", id, strconv.FormatUint(replicationKey, 36))
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marhalledRawHeader,
		Id:        spaceId,
	}
	err = validateCreateSpaceHeaderPayload(rawHeaderWithId)
	require.NoError(t, err)
}

func TestFailedHeaderPayloadForSpaceCreate_InvalidFormatSpaceId(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	spaceHeaderSeed := make([]byte, 32)
	_, err = rand.Read(spaceHeaderSeed)
	require.NoError(t, err)
	spaceHeaderPayload := make([]byte, 32)
	_, err = rand.Read(spaceHeaderPayload)
	require.NoError(t, err)
	replicationKey := rand2.Uint64()
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		Timestamp:          time.Now().Unix(),
		SpaceType:          "SpaceType",
		ReplicationKey:     replicationKey,
		Seed:               spaceHeaderSeed,
		SpaceHeaderPayload: spaceHeaderPayload,
	}
	marhalled, err := header.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marhalled)
	require.NoError(t, err)
	rawHeader := &spacesyncproto.RawSpaceHeader{
		SpaceHeader: marhalled,
		Signature:   signature,
	}
	marhalledRawHeader, err := rawHeader.Marshal()
	require.NoError(t, err)
	id, err := cidutil.NewCidFromBytes(marhalled)
	require.NoError(t, err)
	spaceId := fmt.Sprintf("%s%s", id, strconv.FormatUint(replicationKey, 36))
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marhalledRawHeader,
		Id:        spaceId,
	}
	err = validateCreateSpaceHeaderPayload(rawHeaderWithId)
	assert.EqualErrorf(t, err, ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
}

func TestFailedHeaderPayloadForSpaceCreate_CidIsWrong(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	spaceHeaderSeed := make([]byte, 32)
	_, err = rand.Read(spaceHeaderSeed)
	require.NoError(t, err)
	spaceHeaderPayload := make([]byte, 32)
	_, err = rand.Read(spaceHeaderPayload)
	require.NoError(t, err)
	replicationKey := rand2.Uint64()
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		Timestamp:          time.Now().Unix(),
		SpaceType:          "SpaceType",
		ReplicationKey:     replicationKey,
		Seed:               spaceHeaderSeed,
		SpaceHeaderPayload: spaceHeaderPayload,
	}
	marhalled, err := header.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marhalled)
	require.NoError(t, err)
	rawHeader := &spacesyncproto.RawSpaceHeader{
		SpaceHeader: marhalled,
		Signature:   signature,
	}
	marhalledRawHeader, err := rawHeader.Marshal()
	require.NoError(t, err)
	id := "faisdfjpiocpoakopkop34"
	spaceId := fmt.Sprintf("%s.%s", id, strconv.FormatUint(replicationKey, 36))
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marhalledRawHeader,
		Id:        spaceId,
	}
	err = validateCreateSpaceHeaderPayload(rawHeaderWithId)
	assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
}

func TestFailedHeaderPayloadForSpaceCreate_SignedWithAnotherIdentity(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	spaceHeaderSeed := make([]byte, 32)
	_, err = rand.Read(spaceHeaderSeed)
	require.NoError(t, err)
	spaceHeaderPayload := make([]byte, 32)
	_, err = rand.Read(spaceHeaderPayload)
	require.NoError(t, err)
	replicationKey := rand2.Uint64()
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		Timestamp:          time.Now().Unix(),
		SpaceType:          "SpaceType",
		ReplicationKey:     replicationKey,
		Seed:               spaceHeaderSeed,
		SpaceHeaderPayload: spaceHeaderPayload,
	}
	marhalled, err := header.Marshal()
	require.NoError(t, err)
	anotherAccountKeys, err := accountdata.NewRandom()
	signature, err := anotherAccountKeys.SignKey.Sign(marhalled)
	require.NoError(t, err)
	rawHeader := &spacesyncproto.RawSpaceHeader{
		SpaceHeader: marhalled,
		Signature:   signature,
	}
	marhalledRawHeader, err := rawHeader.Marshal()
	require.NoError(t, err)
	id := "faisdfjpiocpoakopkop34"
	spaceId := fmt.Sprintf("%s.%s", id, strconv.FormatUint(replicationKey, 36))
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marhalledRawHeader,
		Id:        spaceId,
	}
	err = validateCreateSpaceHeaderPayload(rawHeaderWithId)
	assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
}

func TestSuccessAclPayloadSpace(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	readKeyBytes := make([]byte, 32)
	_, err = rand.Read(readKeyBytes)
	require.NoError(t, err)
	readKey, err := accountKeys.SignKey.GetPublic().Encrypt(readKeyBytes)
	require.NoError(t, err)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	rawIdentity, err := accountKeys.SignKey.GetPublic().Raw()
	require.NoError(t, err)
	identitySignature, err := masterKey.Sign(rawIdentity)
	require.NoError(t, err)
	rawMasterKey, err := masterKey.Raw()
	require.NoError(t, err)
	aclRoot := aclrecordproto.AclRoot{
		Identity:          identity,
		MasterKey:         rawMasterKey,
		SpaceId:           "SpaceId",
		EncryptedReadKey:  readKey,
		Timestamp:         time.Now().Unix(),
		IdentitySignature: identitySignature,
	}
	marshalled, err := aclRoot.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marshalled)
	rawAclRecord := &aclrecordproto.RawAclRecord{
		Payload:   marshalled,
		Signature: signature,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	require.NoError(t, err)
	aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
	require.NoError(t, err)
	rawWithId := &aclrecordproto.RawAclRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	err = validateCreateSpaceAclPayload(rawWithId)
	require.NoError(t, err)
}

func TestFailAclPayloadSpace_IncorrectCid(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	readKeyBytes := make([]byte, 32)
	_, err = rand.Read(readKeyBytes)
	require.NoError(t, err)
	readKey, err := accountKeys.SignKey.GetPublic().Encrypt(readKeyBytes)
	require.NoError(t, err)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	rawIdentity, err := accountKeys.SignKey.GetPublic().Raw()
	require.NoError(t, err)
	identitySignature, err := masterKey.Sign(rawIdentity)
	require.NoError(t, err)
	rawMasterKey, err := masterKey.Raw()
	require.NoError(t, err)
	aclRoot := aclrecordproto.AclRoot{
		Identity:          identity,
		MasterKey:         rawMasterKey,
		SpaceId:           "SpaceId",
		EncryptedReadKey:  readKey,
		Timestamp:         time.Now().Unix(),
		IdentitySignature: identitySignature,
	}
	marshalled, err := aclRoot.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marshalled)
	rawAclRecord := &aclrecordproto.RawAclRecord{
		Payload:   marshalled,
		Signature: signature,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	require.NoError(t, err)
	aclHeadId := "rand"
	rawWithId := &aclrecordproto.RawAclRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	err = validateCreateSpaceAclPayload(rawWithId)
	assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
}

func TestFailedAclPayloadSpace_IncorrectSignature(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	readKeyBytes := make([]byte, 32)
	_, err = rand.Read(readKeyBytes)
	require.NoError(t, err)
	readKey, err := accountKeys.SignKey.GetPublic().Encrypt(readKeyBytes)
	require.NoError(t, err)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	rawIdentity, err := accountKeys.SignKey.GetPublic().Raw()
	require.NoError(t, err)
	identitySignature, err := masterKey.Sign(rawIdentity)
	require.NoError(t, err)
	rawMasterKey, err := masterKey.Raw()
	require.NoError(t, err)
	aclRoot := aclrecordproto.AclRoot{
		Identity:          identity,
		MasterKey:         rawMasterKey,
		SpaceId:           "SpaceId",
		EncryptedReadKey:  readKey,
		Timestamp:         time.Now().Unix(),
		IdentitySignature: identitySignature,
	}
	marshalled, err := aclRoot.Marshal()
	require.NoError(t, err)
	rawAclRecord := &aclrecordproto.RawAclRecord{
		Payload:   marshalled,
		Signature: marshalled,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	require.NoError(t, err)
	aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
	require.NoError(t, err)
	rawWithId := &aclrecordproto.RawAclRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	err = validateCreateSpaceAclPayload(rawWithId)
	assert.NotNil(t, err)
	assert.EqualErrorf(t, err, ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", ErrIncorrectSpaceHeader, err)
}

func TestFailedAclPayloadSpace_IncorrectIdentitySignature(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	readKeyBytes := make([]byte, 32)
	_, err = rand.Read(readKeyBytes)
	require.NoError(t, err)
	readKey, err := accountKeys.SignKey.GetPublic().Encrypt(readKeyBytes)
	require.NoError(t, err)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	rawMasterKey, err := masterKey.Raw()
	require.NoError(t, err)
	aclRoot := aclrecordproto.AclRoot{
		Identity:          identity,
		MasterKey:         rawMasterKey,
		SpaceId:           "SpaceId",
		EncryptedReadKey:  readKey,
		Timestamp:         time.Now().Unix(),
		IdentitySignature: identity,
	}
	marshalled, err := aclRoot.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marshalled)
	rawAclRecord := &aclrecordproto.RawAclRecord{
		Payload:   marshalled,
		Signature: signature,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	require.NoError(t, err)
	aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
	require.NoError(t, err)
	rawWithId := &aclrecordproto.RawAclRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	err = validateCreateSpaceAclPayload(rawWithId)
	assert.EqualErrorf(t, err, ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", ErrIncorrectSpaceHeader, err)
}

func TestSuccessSettingsPayloadSpace(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	spaceSettingsSeed := make([]byte, 32)
	_, err = rand.Read(spaceSettingsSeed)
	require.NoError(t, err)
	changePayload := make([]byte, 32)
	_, err = rand.Read(changePayload)
	require.NoError(t, err)
	rootChange := &treechangeproto.RootChange{
		AclHeadId:     "AclHeadId",
		SpaceId:       "SpaceId",
		ChangeType:    "ChangeType",
		Timestamp:     time.Now().Unix(),
		Seed:          spaceSettingsSeed,
		Identity:      identity,
		ChangePayload: changePayload,
	}
	marshalledChange, err := rootChange.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marshalledChange)
	require.NoError(t, err)
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: signature,
	}
	marshalledRawChange, err := raw.Marshal()
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	require.NoError(t, err)
	rawIdChange := &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	err = validateCreateSpaceSettingsPayload(rawIdChange)
	require.NoError(t, err)
}

func TestFailSettingsPayloadSpace_InvalidSignature(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	spaceSettingsSeed := make([]byte, 32)
	_, err = rand.Read(spaceSettingsSeed)
	require.NoError(t, err)
	changePayload := make([]byte, 32)
	_, err = rand.Read(changePayload)
	require.NoError(t, err)
	rootChange := &treechangeproto.RootChange{
		AclHeadId:     "AclHeadId",
		SpaceId:       "SpaceId",
		ChangeType:    "ChangeType",
		Timestamp:     time.Now().Unix(),
		Seed:          spaceSettingsSeed,
		Identity:      identity,
		ChangePayload: changePayload,
	}
	marshalledChange, err := rootChange.Marshal()
	require.NoError(t, err)
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: marshalledChange,
	}
	marshalledRawChange, err := raw.Marshal()
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	require.NoError(t, err)
	rawIdChange := &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	err = validateCreateSpaceSettingsPayload(rawIdChange)
	assert.EqualErrorf(t, err, ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", ErrIncorrectSpaceHeader, err)
}

func TestFailSettingsPayloadSpace_InvalidCid(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	spaceSettingsSeed := make([]byte, 32)
	_, err = rand.Read(spaceSettingsSeed)
	require.NoError(t, err)
	changePayload := make([]byte, 32)
	_, err = rand.Read(changePayload)
	require.NoError(t, err)
	rootChange := &treechangeproto.RootChange{
		AclHeadId:     "AclHeadId",
		SpaceId:       "SpaceId",
		ChangeType:    "ChangeType",
		Timestamp:     time.Now().Unix(),
		Seed:          spaceSettingsSeed,
		Identity:      identity,
		ChangePayload: changePayload,
	}
	marshalledChange, err := rootChange.Marshal()
	require.NoError(t, err)
	signature, err := accountKeys.SignKey.Sign(marshalledChange)
	require.NoError(t, err)
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: signature,
	}
	marshalledRawChange, err := raw.Marshal()
	id := "id"
	require.NoError(t, err)
	rawIdChange := &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	err = validateCreateSpaceSettingsPayload(rawIdChange)
	assert.EqualErrorf(t, err, ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", ErrIncorrectSpaceHeader, err)
}
