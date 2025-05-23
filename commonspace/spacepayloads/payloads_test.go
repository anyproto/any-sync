package spacepayloads

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

func TestSuccessHeaderPayloadForSpaceCreate(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	_, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
	require.NoError(t, err)
	err = ValidateSpaceHeader(rawHeaderWithId, nil)
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
	replicationKey := rand.Uint64()
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
	err = ValidateSpaceHeader(rawHeaderWithId, nil)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
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
	replicationKey := rand.Uint64()
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
	err = ValidateSpaceHeader(rawHeaderWithId, nil)
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
	replicationKey := rand.Uint64()
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
	err = ValidateSpaceHeader(rawHeaderWithId, nil)
	assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
}

func TestSuccessAclPayloadSpace(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	spaceId := "AnySpaceId"
	_, rawWithId, err := rawAclWithId(accountKeys, spaceId)
	require.NoError(t, err)
	validationSpaceId, err := validateCreateSpaceAclPayload(rawWithId)
	require.Equal(t, validationSpaceId, spaceId)
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
	rawMasterKey, err := masterKey.GetPublic().Marshall()
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
	rawAclRecord := &consensusproto.RawRecord{
		Payload:   marshalled,
		Signature: signature,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	require.NoError(t, err)
	aclHeadId := "rand"
	rawWithId := &consensusproto.RawRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	_, err = validateCreateSpaceAclPayload(rawWithId)
	assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
}

func TestFailedAclPayloadSpace_IncorrectSignature(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
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
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	identitySignature, err := masterKey.Sign(rawIdentity)
	require.NoError(t, err)
	rawMasterKey, err := masterKey.GetPublic().Raw()
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
	rawAclRecord := &consensusproto.RawRecord{
		Payload:   marshalled,
		Signature: marshalled,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	require.NoError(t, err)
	aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
	require.NoError(t, err)
	rawWithId := &consensusproto.RawRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	_, err = validateCreateSpaceAclPayload(rawWithId)
	assert.NotNil(t, err)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
}

func TestFailedAclPayloadSpace_IncorrectIdentitySignature(t *testing.T) {
	spaceId := "AnySpaceId"
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	readKeyBytes := make([]byte, 32)
	_, err = rand.Read(readKeyBytes)
	if err != nil {
		return
	}
	readKey, err := accountKeys.SignKey.GetPublic().Encrypt(readKeyBytes)
	if err != nil {
		return
	}
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return
	}
	masterPubKey := masterKey.GetPublic()
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	rawMasterKey, err := masterPubKey.Marshall()
	if err != nil {
		return
	}
	aclRoot := aclrecordproto.AclRoot{
		Identity:          identity,
		MasterKey:         rawMasterKey,
		SpaceId:           spaceId,
		EncryptedReadKey:  readKey,
		Timestamp:         time.Now().Unix(),
		IdentitySignature: identity,
	}
	marshalled, err := aclRoot.Marshal()
	if err != nil {
		return
	}
	signature, err := accountKeys.SignKey.Sign(marshalled)
	rawAclRecord := &consensusproto.RawRecord{
		Payload:   marshalled,
		Signature: signature,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	if err != nil {
		return
	}
	aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
	if err != nil {
		return
	}
	rawWithId := &consensusproto.RawRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	_, err = validateCreateSpaceAclPayload(rawWithId)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
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
	spaceId := "SpaceId"
	rootChange := &treechangeproto.RootChange{
		AclHeadId:     "AclHeadId",
		SpaceId:       spaceId,
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
	_, validationSpaceId, err := validateCreateSpaceSettingsPayload(rawIdChange)
	require.Equal(t, validationSpaceId, spaceId)
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
	_, _, err = validateCreateSpaceSettingsPayload(rawIdChange)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
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
	_, _, err = validateCreateSpaceSettingsPayload(rawIdChange)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
}

func TestSuccessSameIds(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
	require.NoError(t, err)
	aclHeadId, rawAclWithId, err := rawAclWithId(accountKeys, spaceId)
	require.NoError(t, err)
	rawSettingsPayload, err := rawSettingsPayload(accountKeys, spaceId, aclHeadId)
	spacePayload := spacestorage.SpaceStorageCreatePayload{
		AclWithId:           rawAclWithId,
		SpaceHeaderWithId:   rawHeaderWithId,
		SpaceSettingsWithId: rawSettingsPayload,
	}
	err = ValidateSpaceStorageCreatePayload(spacePayload)
	require.NoError(t, err)
}

func TestFailWithAclWrongSpaceId(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
	require.NoError(t, err)
	aclHeadId, rawAclWithId, err := rawAclWithId(accountKeys, "spaceId")
	require.NoError(t, err)
	rawSettingsPayload, err := rawSettingsPayload(accountKeys, spaceId, aclHeadId)
	spacePayload := spacestorage.SpaceStorageCreatePayload{
		AclWithId:           rawAclWithId,
		SpaceHeaderWithId:   rawHeaderWithId,
		SpaceSettingsWithId: rawSettingsPayload,
	}
	err = ValidateSpaceStorageCreatePayload(spacePayload)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
}

func TestFailWithSettingsWrongSpaceId(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
	require.NoError(t, err)
	aclHeadId, rawAclWithId, err := rawAclWithId(accountKeys, spaceId)
	require.NoError(t, err)
	rawSettingsPayload, err := rawSettingsPayload(accountKeys, "spaceId", aclHeadId)
	spacePayload := spacestorage.SpaceStorageCreatePayload{
		AclWithId:           rawAclWithId,
		SpaceHeaderWithId:   rawHeaderWithId,
		SpaceSettingsWithId: rawSettingsPayload,
	}
	err = ValidateSpaceStorageCreatePayload(spacePayload)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
}

func TestFailWithWrongAclHeadIdInSettingsPayload(t *testing.T) {
	accountKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
	require.NoError(t, err)
	_, rawAclWithId, err := rawAclWithId(accountKeys, spaceId)
	require.NoError(t, err)
	rawSettingsPayload, err := rawSettingsPayload(accountKeys, spaceId, "aclHeadId")
	spacePayload := spacestorage.SpaceStorageCreatePayload{
		AclWithId:           rawAclWithId,
		SpaceHeaderWithId:   rawHeaderWithId,
		SpaceSettingsWithId: rawSettingsPayload,
	}
	err = ValidateSpaceStorageCreatePayload(spacePayload)
	assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
}

func rawSettingsPayload(accountKeys *accountdata.AccountKeys, spaceId, aclHeadId string) (rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	spaceSettingsSeed := make([]byte, 32)
	_, err = rand.Read(spaceSettingsSeed)
	if err != nil {
		return
	}
	changePayload := make([]byte, 32)
	_, err = rand.Read(changePayload)
	if err != nil {
		return
	}
	rootChange := &treechangeproto.RootChange{
		AclHeadId:     aclHeadId,
		SpaceId:       spaceId,
		ChangeType:    "ChangeType",
		Timestamp:     time.Now().Unix(),
		Seed:          spaceSettingsSeed,
		Identity:      identity,
		ChangePayload: changePayload,
	}
	marshalledChange, err := rootChange.Marshal()
	if err != nil {
		return
	}
	signature, err := accountKeys.SignKey.Sign(marshalledChange)
	if err != nil {
		return
	}
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: signature,
	}
	marshalledRawChange, err := raw.Marshal()
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	if err != nil {
		return
	}
	rawIdChange = &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}

	return
}

func rawAclWithId(accountKeys *accountdata.AccountKeys, spaceId string) (aclHeadId string, rawWithId *consensusproto.RawRecordWithId, err error) {
	// TODO: use same storage creation methods as we use in spaces
	readKeyBytes := make([]byte, 32)
	_, err = rand.Read(readKeyBytes)
	if err != nil {
		return
	}
	readKey, err := accountKeys.SignKey.GetPublic().Encrypt(readKeyBytes)
	if err != nil {
		return
	}
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	masterPubKey := masterKey.GetPublic()
	rawIdentity, err := accountKeys.SignKey.GetPublic().Raw()
	if err != nil {
		return
	}
	identitySignature, err := masterKey.Sign(rawIdentity)
	if err != nil {
		return
	}
	rawMasterKey, err := masterPubKey.Marshall()
	if err != nil {
		return
	}
	aclRoot := aclrecordproto.AclRoot{
		Identity:          identity,
		MasterKey:         rawMasterKey,
		SpaceId:           spaceId,
		EncryptedReadKey:  readKey,
		Timestamp:         time.Now().Unix(),
		IdentitySignature: identitySignature,
	}
	marshalled, err := aclRoot.Marshal()
	if err != nil {
		return
	}
	signature, err := accountKeys.SignKey.Sign(marshalled)
	rawAclRecord := &consensusproto.RawRecord{
		Payload:   marshalled,
		Signature: signature,
	}
	marshalledRaw, err := rawAclRecord.Marshal()
	if err != nil {
		return
	}
	aclHeadId, err = cidutil.NewCidFromBytes(marshalledRaw)
	if err != nil {
		return
	}
	rawWithId = &consensusproto.RawRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}

	return
}

func rawHeaderWithId(accountKeys *accountdata.AccountKeys) (spaceId string, rawWithId *spacesyncproto.RawSpaceHeaderWithId, err error) {
	// TODO: use same storage creation methods as we use in spaces
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	spaceHeaderSeed := make([]byte, 32)
	_, err = rand.Read(spaceHeaderSeed)
	if err != nil {
		return
	}
	spaceHeaderPayload := make([]byte, 32)
	_, err = rand.Read(spaceHeaderPayload)
	if err != nil {
		return
	}
	replicationKey := rand.Uint64()
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		Timestamp:          time.Now().Unix(),
		SpaceType:          "SpaceType",
		ReplicationKey:     replicationKey,
		Seed:               spaceHeaderSeed,
		SpaceHeaderPayload: spaceHeaderPayload,
	}
	marhalled, err := header.Marshal()
	if err != nil {
		return
	}
	signature, err := accountKeys.SignKey.Sign(marhalled)
	if err != nil {
		return
	}
	rawHeader := &spacesyncproto.RawSpaceHeader{
		SpaceHeader: marhalled,
		Signature:   signature,
	}
	marshalledRawHeader, err := rawHeader.Marshal()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalledRawHeader)
	if err != nil {
		return
	}
	spaceId = fmt.Sprintf("%s.%s", id, strconv.FormatUint(replicationKey, 36))
	rawWithId = &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marshalledRawHeader,
		Id:        spaceId,
	}

	return
}
