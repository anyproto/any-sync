package commonspace

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"hash/fnv"
	"math/rand"
	"time"
)

func storagePayloadForSpaceCreate(payload SpaceCreatePayload) (storagePayload storage.SpaceStorageCreatePayload, err error) {
	// unmarshalling signing and encryption keys
	identity, err := payload.SigningKey.GetPublic().Raw()
	if err != nil {
		return
	}
	encPubKey, err := payload.EncryptionKey.GetPublic().Raw()
	if err != nil {
		return
	}

	// preparing header and space id
	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)
	if err != nil {
		return
	}
	header := &spacesyncproto.SpaceHeader{
		Identity:       identity,
		Timestamp:      time.Now().UnixNano(),
		SpaceType:      payload.SpaceType,
		ReplicationKey: payload.ReplicationKey,
		Seed:           bytes,
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
	id, err := cid.NewCIDFromBytes(marshalled)
	spaceId := NewSpaceId(id, payload.ReplicationKey)
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marshalled,
		Id:        spaceId,
	}

	// encrypting read key
	hasher := fnv.New64()
	_, err = hasher.Write(payload.ReadKey)
	if err != nil {
		return
	}
	readKeyHash := hasher.Sum64()
	encReadKey, err := payload.EncryptionKey.GetPublic().Encrypt(payload.ReadKey)
	if err != nil {
		return
	}

	// preparing acl
	aclRoot := &aclrecordproto.ACLRoot{
		Identity:           identity,
		EncryptionKey:      encPubKey,
		SpaceId:            spaceId,
		EncryptedReadKey:   encReadKey,
		DerivationScheme:   "",
		CurrentReadKeyHash: readKeyHash,
		Timestamp:          time.Now().UnixNano(),
	}
	rawWithId, err := marshalACLRoot(aclRoot, payload.SigningKey)
	if err != nil {
		return
	}

	// creating storage
	storagePayload = storage.SpaceStorageCreatePayload{
		RecWithId:         rawWithId,
		SpaceHeaderWithId: rawHeaderWithId,
	}
	return
}

func storagePayloadForSpaceDerive(payload SpaceDerivePayload) (storagePayload storage.SpaceStorageCreatePayload, err error) {
	// unmarshalling signing and encryption keys
	identity, err := payload.SigningKey.GetPublic().Raw()
	if err != nil {
		return
	}
	signPrivKey, err := payload.SigningKey.Raw()
	if err != nil {
		return
	}
	encPubKey, err := payload.EncryptionKey.GetPublic().Raw()
	if err != nil {
		return
	}
	encPrivKey, err := payload.EncryptionKey.Raw()
	if err != nil {
		return
	}

	// preparing replication key
	hasher := fnv.New64()
	_, err = hasher.Write(identity)
	if err != nil {
		return
	}
	repKey := hasher.Sum64()

	// preparing header and space id
	header := &spacesyncproto.SpaceHeader{
		Identity:       identity,
		SpaceType:      SpaceTypeDerived,
		ReplicationKey: repKey,
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
	id, err := cid.NewCIDFromBytes(marshalled)
	spaceId := NewSpaceId(id, repKey)
	rawHeaderWithId := &spacesyncproto.RawSpaceHeaderWithId{
		RawHeader: marshalled,
		Id:        spaceId,
	}

	// deriving and encrypting read key
	readKey, err := aclrecordproto.ACLReadKeyDerive(signPrivKey, encPrivKey)
	if err != nil {
		return
	}
	hasher = fnv.New64()
	_, err = hasher.Write(readKey.Bytes())
	if err != nil {
		return
	}
	readKeyHash := hasher.Sum64()
	encReadKey, err := payload.EncryptionKey.GetPublic().Encrypt(readKey.Bytes())
	if err != nil {
		return
	}

	// preparing acl
	aclRoot := &aclrecordproto.ACLRoot{
		Identity:           identity,
		EncryptionKey:      encPubKey,
		SpaceId:            spaceId,
		EncryptedReadKey:   encReadKey,
		DerivationScheme:   "",
		CurrentReadKeyHash: readKeyHash,
		Timestamp:          time.Now().UnixNano(),
	}
	rawWithId, err := marshalACLRoot(aclRoot, payload.SigningKey)
	if err != nil {
		return
	}

	// creating storage
	storagePayload = storage.SpaceStorageCreatePayload{
		RecWithId:         rawWithId,
		SpaceHeaderWithId: rawHeaderWithId,
	}
	return
}

func marshalACLRoot(aclRoot *aclrecordproto.ACLRoot, key signingkey.PrivKey) (rawWithId *aclrecordproto.RawACLRecordWithId, err error) {
	marshalledRoot, err := aclRoot.Marshal()
	if err != nil {
		return
	}
	signature, err := key.Sign(marshalledRoot)
	if err != nil {
		return
	}
	raw := &aclrecordproto.RawACLRecord{
		Payload:   marshalledRoot,
		Signature: signature,
	}
	marshalledRaw, err := raw.Marshal()
	if err != nil {
		return
	}
	aclHeadId, err := cid.NewCIDFromBytes(marshalledRaw)
	if err != nil {
		return
	}
	rawWithId = &aclrecordproto.RawACLRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	return
}
