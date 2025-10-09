package spacepayloads

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand"
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

func TestValidateSpaceHeader(t *testing.T) {
	t.Run("success v0 header", func(t *testing.T) {
		accountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		_, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
		require.NoError(t, err)
		_, err = ValidateSpaceHeader(rawHeaderWithId, nil, nil, nil)
		require.NoError(t, err)
	})

	t.Run("invalid format space id (no dot)", func(t *testing.T) {
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
		replicationKey := mrand.Uint64()
		header := &spacesyncproto.SpaceHeader{
			Identity:           identity,
			Timestamp:          time.Now().Unix(),
			SpaceType:          "SpaceType",
			ReplicationKey:     replicationKey,
			Seed:               spaceHeaderSeed,
			SpaceHeaderPayload: spaceHeaderPayload,
		}
		marhalled, err := header.MarshalVT()
		require.NoError(t, err)
		signature, err := accountKeys.SignKey.Sign(marhalled)
		require.NoError(t, err)
		rawHeader := &spacesyncproto.RawSpaceHeader{
			SpaceHeader: marhalled,
			Signature:   signature,
		}
		marhalledRawHeader, err := rawHeader.MarshalVT()
		require.NoError(t, err)
		id, err := cidutil.NewCidFromBytes(marhalled)
		require.NoError(t, err)
		spaceId := fmt.Sprintf("%s%s", id, strconv.FormatUint(replicationKey, 36))
		rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
			RawHeader: marhalledRawHeader,
			Id:        spaceId,
		}
		_, err = ValidateSpaceHeader(rawHeaderWithId, nil, nil, nil)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})

	t.Run("wrong cid", func(t *testing.T) {
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
		replicationKey := mrand.Uint64()
		header := &spacesyncproto.SpaceHeader{
			Identity:           identity,
			Timestamp:          time.Now().Unix(),
			SpaceType:          "SpaceType",
			ReplicationKey:     replicationKey,
			Seed:               spaceHeaderSeed,
			SpaceHeaderPayload: spaceHeaderPayload,
		}
		marhalled, err := header.MarshalVT()
		require.NoError(t, err)
		signature, err := accountKeys.SignKey.Sign(marhalled)
		require.NoError(t, err)
		rawHeader := &spacesyncproto.RawSpaceHeader{
			SpaceHeader: marhalled,
			Signature:   signature,
		}
		marhalledRawHeader, err := rawHeader.MarshalVT()
		require.NoError(t, err)
		id := "faisdfjpiocpoakopkop34"
		spaceId := fmt.Sprintf("%s.%s", id, strconv.FormatUint(replicationKey, 36))
		rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
			RawHeader: marhalledRawHeader,
			Id:        spaceId,
		}
		_, err = ValidateSpaceHeader(rawHeaderWithId, nil, nil, nil)
		assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
	})

	t.Run("signed with another identity", func(t *testing.T) {
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
		replicationKey := mrand.Uint64()
		header := &spacesyncproto.SpaceHeader{
			Identity:           identity,
			Timestamp:          time.Now().Unix(),
			SpaceType:          "SpaceType",
			ReplicationKey:     replicationKey,
			Seed:               spaceHeaderSeed,
			SpaceHeaderPayload: spaceHeaderPayload,
		}
		marhalled, err := header.MarshalVT()
		require.NoError(t, err)
		anotherAccountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		signature, err := anotherAccountKeys.SignKey.Sign(marhalled)
		require.NoError(t, err)
		rawHeader := &spacesyncproto.RawSpaceHeader{
			SpaceHeader: marhalled,
			Signature:   signature,
		}
		marhalledRawHeader, err := rawHeader.MarshalVT()
		require.NoError(t, err)
		id := "faisdfjpiocpoakopkop34"
		spaceId := fmt.Sprintf("%s.%s", id, strconv.FormatUint(replicationKey, 36))
		rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
			RawHeader: marhalledRawHeader,
			Id:        spaceId,
		}
		_, err = ValidateSpaceHeader(rawHeaderWithId, nil, nil, nil)
		assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
	})
}

func TestValidateCreateSpaceAclPayload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		accountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		spaceId := "AnySpaceId"
		_, rawWithId, err := rawAclWithId(accountKeys, spaceId)
		require.NoError(t, err)
		validationSpaceId, err := validateCreateSpaceAclPayload(rawWithId)
		require.Equal(t, validationSpaceId, spaceId)
		require.NoError(t, err)
	})

	t.Run("incorrect cid", func(t *testing.T) {
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
		marshalled, err := aclRoot.MarshalVT()
		require.NoError(t, err)
		signature, err := accountKeys.SignKey.Sign(marshalled)
		require.NoError(t, err)
		rawAclRecord := &consensusproto.RawRecord{
			Payload:   marshalled,
			Signature: signature,
		}
		marshalledRaw, err := rawAclRecord.MarshalVT()
		require.NoError(t, err)
		aclHeadId := "rand"
		rawWithId := &consensusproto.RawRecordWithId{
			Payload: marshalledRaw,
			Id:      aclHeadId,
		}
		_, err = validateCreateSpaceAclPayload(rawWithId)
		assert.EqualErrorf(t, err, objecttree.ErrIncorrectCid.Error(), "Error should be: %v, got: %v", objecttree.ErrIncorrectCid, err)
	})

	t.Run("incorrect signature", func(t *testing.T) {
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
		require.NoError(t, err)
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
		marshalled, err := aclRoot.MarshalVT()
		require.NoError(t, err)
		rawAclRecord := &consensusproto.RawRecord{
			Payload:   marshalled,
			Signature: marshalled,
		}
		marshalledRaw, err := rawAclRecord.MarshalVT()
		require.NoError(t, err)
		aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
		require.NoError(t, err)
		rawWithId := &consensusproto.RawRecordWithId{
			Payload: marshalledRaw,
			Id:      aclHeadId,
		}
		_, err = validateCreateSpaceAclPayload(rawWithId)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})

	t.Run("incorrect identity signature", func(t *testing.T) {
		spaceId := "AnySpaceId"
		accountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		readKeyBytes := make([]byte, 32)
		_, err = rand.Read(readKeyBytes)
		require.NoError(t, err)
		readKey, err := accountKeys.SignKey.GetPublic().Encrypt(readKeyBytes)
		require.NoError(t, err)
		masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		masterPubKey := masterKey.GetPublic()
		identity, err := accountKeys.SignKey.GetPublic().Marshall()
		require.NoError(t, err)
		rawMasterKey, err := masterPubKey.Marshall()
		require.NoError(t, err)
		aclRoot := aclrecordproto.AclRoot{
			Identity:          identity,
			MasterKey:         rawMasterKey,
			SpaceId:           spaceId,
			EncryptedReadKey:  readKey,
			Timestamp:         time.Now().Unix(),
			IdentitySignature: identity, // wrong on purpose
		}
		marshalled, err := aclRoot.MarshalVT()
		require.NoError(t, err)
		signature, err := accountKeys.SignKey.Sign(marshalled)
		require.NoError(t, err)
		rawAclRecord := &consensusproto.RawRecord{
			Payload:   marshalled,
			Signature: signature,
		}
		marshalledRaw, err := rawAclRecord.MarshalVT()
		require.NoError(t, err)
		aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
		require.NoError(t, err)
		rawWithId := &consensusproto.RawRecordWithId{
			Payload: marshalledRaw,
			Id:      aclHeadId,
		}
		_, err = validateCreateSpaceAclPayload(rawWithId)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})
}

func TestValidateCreateSpaceSettingsPayload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
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
		marshalledChange, err := rootChange.MarshalVT()
		require.NoError(t, err)
		signature, err := accountKeys.SignKey.Sign(marshalledChange)
		require.NoError(t, err)
		raw := &treechangeproto.RawTreeChange{
			Payload:   marshalledChange,
			Signature: signature,
		}
		marshalledRawChange, err := raw.MarshalVT()
		require.NoError(t, err)
		id, err := cidutil.NewCidFromBytes(marshalledRawChange)
		require.NoError(t, err)
		rawIdChange := &treechangeproto.RawTreeChangeWithId{
			RawChange: marshalledRawChange,
			Id:        id,
		}
		_, validationSpaceId, err := validateCreateSpaceSettingsPayload(rawIdChange)
		require.Equal(t, validationSpaceId, spaceId)
		require.NoError(t, err)
	})

	t.Run("invalid signature", func(t *testing.T) {
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
		marshalledChange, err := rootChange.MarshalVT()
		require.NoError(t, err)
		raw := &treechangeproto.RawTreeChange{
			Payload:   marshalledChange,
			Signature: marshalledChange,
		}
		marshalledRawChange, err := raw.MarshalVT()
		require.NoError(t, err)
		id, err := cidutil.NewCidFromBytes(marshalledRawChange)
		require.NoError(t, err)
		rawIdChange := &treechangeproto.RawTreeChangeWithId{
			RawChange: marshalledRawChange,
			Id:        id,
		}
		_, _, err = validateCreateSpaceSettingsPayload(rawIdChange)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})

	t.Run("invalid cid", func(t *testing.T) {
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
		marshalledChange, err := rootChange.MarshalVT()
		require.NoError(t, err)
		signature, err := accountKeys.SignKey.Sign(marshalledChange)
		require.NoError(t, err)
		raw := &treechangeproto.RawTreeChange{
			Payload:   marshalledChange,
			Signature: signature,
		}
		marshalledRawChange, err := raw.MarshalVT()
		require.NoError(t, err)
		id := "id" // wrong on purpose
		rawIdChange := &treechangeproto.RawTreeChangeWithId{
			RawChange: marshalledRawChange,
			Id:        id,
		}
		_, _, err = validateCreateSpaceSettingsPayload(rawIdChange)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})
}

func TestValidateSpaceStorageCreatePayload(t *testing.T) {
	t.Run("success same ids (v0)", func(t *testing.T) {
		accountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
		require.NoError(t, err)
		aclHeadId, rawAclWithId, err := rawAclWithId(accountKeys, spaceId)
		require.NoError(t, err)
		rawSettingsPayload, err := rawSettingsPayload(accountKeys, spaceId, aclHeadId)
		require.NoError(t, err)
		spacePayload := spacestorage.SpaceStorageCreatePayload{
			AclWithId:           rawAclWithId,
			SpaceHeaderWithId:   rawHeaderWithId,
			SpaceSettingsWithId: rawSettingsPayload,
		}
		err = ValidateSpaceStorageCreatePayload(spacePayload)
		require.NoError(t, err)
	})

	t.Run("fail: acl wrong spaceId", func(t *testing.T) {
		accountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
		require.NoError(t, err)
		aclHeadId, rawAclWithId, err := rawAclWithId(accountKeys, "spaceId")
		require.NoError(t, err)
		rawSettingsPayload, err := rawSettingsPayload(accountKeys, spaceId, aclHeadId)
		require.NoError(t, err)
		spacePayload := spacestorage.SpaceStorageCreatePayload{
			AclWithId:           rawAclWithId,
			SpaceHeaderWithId:   rawHeaderWithId,
			SpaceSettingsWithId: rawSettingsPayload,
		}
		err = ValidateSpaceStorageCreatePayload(spacePayload)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})

	t.Run("fail: settings wrong spaceId", func(t *testing.T) {
		accountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
		require.NoError(t, err)
		aclHeadId, rawAclWithId, err := rawAclWithId(accountKeys, spaceId)
		require.NoError(t, err)
		rawSettingsPayload, err := rawSettingsPayload(accountKeys, "spaceId", aclHeadId)
		require.NoError(t, err)
		spacePayload := spacestorage.SpaceStorageCreatePayload{
			AclWithId:           rawAclWithId,
			SpaceHeaderWithId:   rawHeaderWithId,
			SpaceSettingsWithId: rawSettingsPayload,
		}
		err = ValidateSpaceStorageCreatePayload(spacePayload)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})

	t.Run("fail: wrong AclHeadId in settings", func(t *testing.T) {
		accountKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		spaceId, rawHeaderWithId, err := rawHeaderWithId(accountKeys)
		require.NoError(t, err)
		_, rawAclWithId, err := rawAclWithId(accountKeys, spaceId)
		require.NoError(t, err)
		rawSettingsPayload, err := rawSettingsPayload(accountKeys, spaceId, "aclHeadId")
		require.NoError(t, err)
		spacePayload := spacestorage.SpaceStorageCreatePayload{
			AclWithId:           rawAclWithId,
			SpaceHeaderWithId:   rawHeaderWithId,
			SpaceSettingsWithId: rawSettingsPayload,
		}
		err = ValidateSpaceStorageCreatePayload(spacePayload)
		assert.EqualErrorf(t, err, spacestorage.ErrIncorrectSpaceHeader.Error(), "Error should be: %v, got: %v", spacestorage.ErrIncorrectSpaceHeader, err)
	})
}

func TestStoragePayloadForOneToOneSpace(t *testing.T) {
	t.Run("makeOneToOneInfo", func(t *testing.T) {
		aSk, aPk, _ := crypto.GenerateRandomEd25519KeyPair()
		_, bPk, _ := crypto.GenerateRandomEd25519KeyPair()
		sharedSk, _ := crypto.GenerateSharedKey(aSk, bPk, crypto.AnysyncOneToOneSpacePath)

		oneToOneInfo, err := makeOneToOneInfo(sharedSk, aPk, bPk)
		ownerPk, err := crypto.UnmarshalEd25519PublicKeyProto(oneToOneInfo.Owner)
		require.NoError(t, err)
		assert.True(t, ownerPk.Equals(sharedSk.GetPublic()))

		writer0Pk, err := crypto.UnmarshalEd25519PublicKeyProto(oneToOneInfo.Writers[0])
		require.NoError(t, err)
		writer1Pk, err := crypto.UnmarshalEd25519PublicKeyProto(oneToOneInfo.Writers[1])
		require.NoError(t, err)

		assert.True(t, writer0Pk.Equals(aPk) || writer1Pk.Equals(aPk))
		assert.True(t, writer0Pk.Equals(bPk) || writer1Pk.Equals(bPk))
		assert.False(t, writer0Pk.Equals(writer1Pk))
	})

	t.Run("StoragePayloadForOneToOneSpace", func(t *testing.T) {

	})

}
func TestValidateSpaceHeader_OneToOne(t *testing.T) {
	// ValidateSpaceHeader
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
	marshalledChange, err := rootChange.MarshalVT()
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
	marshalledRawChange, err := raw.MarshalVT()
	if err != nil {
		return
	}
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
	marshalled, err := aclRoot.MarshalVT()
	if err != nil {
		return
	}
	signature, err := accountKeys.SignKey.Sign(marshalled)
	if err != nil {
		return
	}
	rawAclRecord := &consensusproto.RawRecord{
		Payload:   marshalled,
		Signature: signature,
	}
	marshalledRaw, err := rawAclRecord.MarshalVT()
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
	replicationKey := mrand.Uint64()
	header := &spacesyncproto.SpaceHeader{
		Identity:           identity,
		Timestamp:          time.Now().Unix(),
		SpaceType:          "SpaceType",
		ReplicationKey:     replicationKey,
		Seed:               spaceHeaderSeed,
		SpaceHeaderPayload: spaceHeaderPayload,
	}
	marhalled, err := header.MarshalVT()
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
	marshalledRawHeader, err := rawHeader.MarshalVT()
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

func rawHeaderWithIdV1HeaderOnly(accountKeys *accountdata.AccountKeys) (spaceId string, rawWithId *spacesyncproto.RawSpaceHeaderWithId, err error) {
	identity, err := accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	spaceHeaderSeed := make([]byte, 32)
	_, err = rand.Read(spaceHeaderSeed)
	if err != nil {
		return
	}
	replicationKey := mrand.Uint64()
	header := &spacesyncproto.SpaceHeader{
		Identity:       identity,
		Timestamp:      time.Now().Unix(),
		SpaceType:      "SpaceType",
		ReplicationKey: replicationKey,
		Seed:           spaceHeaderSeed,
		Version:        spacesyncproto.SpaceHeaderVersion_SpaceHeaderVersion1,
		// AclPayload/SettingPayload intentionally omitted for header-only case
	}
	marhalled, err := header.MarshalVT()
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
	marshalledRawHeader, err := rawHeader.MarshalVT()
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

func TestStoragePayloadForSpaceCreate(t *testing.T) {
	t.Run("success: builds valid v0 payload", func(t *testing.T) {
		acc, err := accountdata.NewRandom()
		require.NoError(t, err)

		// keys
		master, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		metaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)

		readKey, _ := crypto.NewRandomAES()

		pl := SpaceCreatePayload{
			SigningKey:     acc.SignKey,
			SpaceType:      "test.space",
			ReplicationKey: mrand.Uint64(),
			SpacePayload:   randBytes(16),
			MasterKey:      master,
			ReadKey:        readKey,
			MetadataKey:    metaKey,
			Metadata:       randBytes(8),
		}

		out, err := StoragePayloadForSpaceCreate(pl)
		require.NoError(t, err)

		// Full v0 payload should validate as a set.
		require.NoError(t, ValidateSpaceStorageCreatePayload(out))
	})
}

func TestStoragePayloadForSpaceCreateV1(t *testing.T) {
	t.Run("success: builds valid v1 payload (full)", func(t *testing.T) {
		acc, err := accountdata.NewRandom()
		require.NoError(t, err)

		master, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		metaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		readKey, _ := crypto.NewRandomAES()

		pl := SpaceCreatePayload{
			SigningKey:     acc.SignKey,
			SpaceType:      "test.space",
			ReplicationKey: mrand.Uint64(),
			SpacePayload:   randBytes(12),
			MasterKey:      master,
			ReadKey:        readKey,
			MetadataKey:    metaKey,
			Metadata:       randBytes(6),
		}

		out, err := StoragePayloadForSpaceCreateV1(pl)
		require.NoError(t, err)

		// Full v1 payload should validate as a set.
		require.NoError(t, ValidateSpaceStorageCreatePayload(out))
	})

	t.Run("success: v1 header-only validates", func(t *testing.T) {
		acc, err := accountdata.NewRandom()
		require.NoError(t, err)

		master, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		metaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		readKey, _ := crypto.NewRandomAES()

		pl := SpaceCreatePayload{
			SigningKey:     acc.SignKey,
			SpaceType:      "test.space",
			ReplicationKey: mrand.Uint64(),
			SpacePayload:   randBytes(12),
			MasterKey:      master,
			ReadKey:        readKey,
			MetadataKey:    metaKey,
			Metadata:       randBytes(6),
		}

		full, err := StoragePayloadForSpaceCreateV1(pl)
		require.NoError(t, err)

		require.NoError(t, ValidateSpaceStorageCreatePayload(full))
	})
}

func TestStoragePayloadForSpaceDerive(t *testing.T) {
	t.Run("success: builds valid derived space payload", func(t *testing.T) {
		acc, err := accountdata.NewRandom()
		require.NoError(t, err)

		master, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)

		pl := SpaceDerivePayload{
			SigningKey:   acc.SignKey,
			MasterKey:    master,
			SpaceType:    "derived.space",
			SpacePayload: randBytes(10),
		}

		out, err := StoragePayloadForSpaceDerive(pl)
		require.NoError(t, err)

		// Derived payload returns header + ACL + settings; should validate.
		require.NoError(t, ValidateSpaceStorageCreatePayload(out))
	})
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}
