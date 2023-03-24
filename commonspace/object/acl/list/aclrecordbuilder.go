package list

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
)

type AclRecordBuilder interface {
	FromRaw(rawIdRecord *aclrecordproto.RawAclRecordWithId) (rec *AclRecord, err error)
}

type aclRecordBuilder struct {
	id         string
	keyStorage crypto.KeyStorage
}

func newAclRecordBuilder(id string, keyStorage crypto.KeyStorage) AclRecordBuilder {
	return &aclRecordBuilder{
		id:         id,
		keyStorage: keyStorage,
	}
}

// TODO: update with new logic
//func (a *aclRecordBuilder) BuildUserJoin(acceptPrivKeyBytes []byte, encSymKeyBytes []byte, state *AclState) (rec *aclrecordproto.RawAclRecord, err error) {
//	acceptPrivKey, err := crypto.NewSigningEd25519PrivKeyFromBytes(acceptPrivKeyBytes)
//	if err != nil {
//		return
//	}
//	acceptPubKeyBytes, err := acceptPrivKey.GetPublic().Raw()
//	if err != nil {
//		return
//	}
//	encSymKey, err := crypto.UnmarshallAESKey(encSymKeyBytes)
//	if err != nil {
//		return
//	}
//
//	invite, err := state.Invite(acceptPubKeyBytes)
//	if err != nil {
//		return
//	}
//
//	encPrivKey, signPrivKey := state.UserKeys()
//	var symKeys [][]byte
//	for _, rk := range invite.EncryptedReadKeys {
//		dec, err := encSymKey.Decrypt(rk)
//		if err != nil {
//			return nil, err
//		}
//		newEnc, err := encPrivKey.GetPublic().Encrypt(dec)
//		if err != nil {
//			return nil, err
//		}
//		symKeys = append(symKeys, newEnc)
//	}
//	idSignature, err := acceptPrivKey.Sign(state.Identity())
//	if err != nil {
//		return
//	}
//	encPubKeyBytes, err := encPrivKey.GetPublic().Raw()
//	if err != nil {
//		return
//	}
//
//	userJoin := &aclrecordproto.AclUserJoin{
//		Identity:          state.Identity(),
//		EncryptionKey:     encPubKeyBytes,
//		AcceptSignature:   idSignature,
//		AcceptPubKey:      acceptPubKeyBytes,
//		EncryptedReadKeys: symKeys,
//	}
//	aclData := &aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{
//		{Value: &aclrecordproto.AclContentValue_UserJoin{UserJoin: userJoin}},
//	}}
//	marshalledJoin, err := aclData.Marshal()
//	if err != nil {
//		return
//	}
//	aclRecord := &aclrecordproto.AclRecord{
//		PrevId:             state.LastRecordId(),
//		Identity:           state.Identity(),
//		Data:               marshalledJoin,
//		CurrentReadKeyHash: state.CurrentReadKeyId(),
//		Timestamp:          time.Now().Unix(),
//	}
//	marshalledRecord, err := aclRecord.Marshal()
//	if err != nil {
//		return
//	}
//	recSignature, err := signPrivKey.Sign(marshalledRecord)
//	if err != nil {
//		return
//	}
//	rec = &aclrecordproto.RawAclRecord{
//		Payload:   marshalledRecord,
//		Signature: recSignature,
//	}
//	return
//}

func (a *aclRecordBuilder) FromRaw(rawIdRecord *aclrecordproto.RawAclRecordWithId) (rec *AclRecord, err error) {
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
