package commonspace

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/crypto"
	"hash/fnv"
	"math/rand"
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
