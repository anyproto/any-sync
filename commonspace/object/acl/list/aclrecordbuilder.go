package list

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"time"
)

type RootContent struct {
	PrivKey          crypto.PrivKey
	MasterKey        crypto.PrivKey
	SpaceId          string
	EncryptedReadKey []byte
}

type AclRecordBuilder interface {
	Unmarshall(rawIdRecord *aclrecordproto.RawAclRecordWithId) (rec *AclRecord, err error)
	BuildRoot(content RootContent) (rec *aclrecordproto.RawAclRecordWithId, err error)
}

type aclRecordBuilder struct {
	id         string
	keyStorage crypto.KeyStorage
}

func NewAclRecordBuilder(id string, keyStorage crypto.KeyStorage) AclRecordBuilder {
	return &aclRecordBuilder{
		id:         id,
		keyStorage: keyStorage,
	}
}

func (a *aclRecordBuilder) Unmarshall(rawIdRecord *aclrecordproto.RawAclRecordWithId) (rec *AclRecord, err error) {
	var (
		rawRec = &aclrecordproto.RawAclRecord{}
		pubKey crypto.PubKey
	)
	err = proto.Unmarshal(rawIdRecord.Payload, rawRec)
	if err != nil {
		return
	}
	if rawIdRecord.Id == a.id {
		aclRoot := &aclrecordproto.AclRoot{}
		err = proto.Unmarshal(rawRec.Payload, aclRoot)
		if err != nil {
			return
		}
		pubKey, err = a.keyStorage.PubKeyFromProto(aclRoot.Identity)
		if err != nil {
			return
		}
		rec = &AclRecord{
			Id:        rawIdRecord.Id,
			ReadKeyId: rawIdRecord.Id,
			Timestamp: aclRoot.Timestamp,
			Signature: rawRec.Signature,
			Identity:  pubKey,
			Model:     aclRoot,
		}
	} else {
		aclRecord := &aclrecordproto.AclRecord{}
		err = proto.Unmarshal(rawRec.Payload, aclRecord)
		if err != nil {
			return
		}
		pubKey, err = a.keyStorage.PubKeyFromProto(aclRecord.Identity)
		if err != nil {
			return
		}
		rec = &AclRecord{
			Id:        rawIdRecord.Id,
			PrevId:    aclRecord.PrevId,
			ReadKeyId: aclRecord.ReadKeyId,
			Timestamp: aclRecord.Timestamp,
			Data:      aclRecord.Data,
			Signature: rawRec.Signature,
			Identity:  pubKey,
		}
	}

	err = verifyRaw(pubKey, rawRec, rawIdRecord)
	return
}

func (a *aclRecordBuilder) BuildRoot(content RootContent) (rec *aclrecordproto.RawAclRecordWithId, err error) {
	identity, err := content.PrivKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	masterKey, err := content.MasterKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	identitySignature, err := content.MasterKey.Sign(identity)
	if err != nil {
		return
	}
	var timestamp int64
	if content.EncryptedReadKey != nil {
		timestamp = time.Now().Unix()
	}
	aclRoot := &aclrecordproto.AclRoot{
		Identity:          identity,
		SpaceId:           content.SpaceId,
		EncryptedReadKey:  content.EncryptedReadKey,
		MasterKey:         masterKey,
		IdentitySignature: identitySignature,
		Timestamp:         timestamp,
	}
	return marshalAclRoot(aclRoot, content.PrivKey)
}

func verifyRaw(
	pubKey crypto.PubKey,
	rawRec *aclrecordproto.RawAclRecord,
	recWithId *aclrecordproto.RawAclRecordWithId) (err error) {
	// verifying signature
	res, err := pubKey.Verify(rawRec.Payload, rawRec.Signature)
	if err != nil {
		return
	}
	if !res {
		err = ErrInvalidSignature
		return
	}

	// verifying ID
	if !cidutil.VerifyCid(recWithId.Payload, recWithId.Id) {
		err = ErrIncorrectCID
	}
	return
}

func marshalAclRoot(aclRoot *aclrecordproto.AclRoot, key crypto.PrivKey) (rawWithId *aclrecordproto.RawAclRecordWithId, err error) {
	marshalledRoot, err := aclRoot.Marshal()
	if err != nil {
		return
	}
	signature, err := key.Sign(marshalledRoot)
	if err != nil {
		return
	}
	raw := &aclrecordproto.RawAclRecord{
		Payload:   marshalledRoot,
		Signature: signature,
	}
	marshalledRaw, err := raw.Marshal()
	if err != nil {
		return
	}
	aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
	if err != nil {
		return
	}
	rawWithId = &aclrecordproto.RawAclRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	return
}
