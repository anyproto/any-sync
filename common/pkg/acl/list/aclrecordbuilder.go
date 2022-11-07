package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"github.com/gogo/protobuf/proto"
	"time"
)

// remove interface
type ACLRecordBuilder interface {
	ConvertFromRaw(rawIdRecord *aclrecordproto.RawACLRecordWithId) (rec *ACLRecord, err error)
	BuildUserJoin(acceptPrivKeyBytes []byte, encSymKeyBytes []byte, state *ACLState) (rec *aclrecordproto.RawACLRecord, err error)
}

type aclRecordBuilder struct {
	id       string
	keychain *common.Keychain
}

func newACLRecordBuilder(id string, keychain *common.Keychain) ACLRecordBuilder {
	return &aclRecordBuilder{
		id:       id,
		keychain: keychain,
	}
}

func (a *aclRecordBuilder) BuildUserJoin(acceptPrivKeyBytes []byte, encSymKeyBytes []byte, state *ACLState) (rec *aclrecordproto.RawACLRecord, err error) {
	acceptPrivKey, err := signingkey.NewSigningEd25519PrivKeyFromBytes(acceptPrivKeyBytes)
	if err != nil {
		return
	}
	acceptPubKeyBytes, err := acceptPrivKey.GetPublic().Raw()
	if err != nil {
		return
	}
	encSymKey, err := symmetric.FromBytes(encSymKeyBytes)
	if err != nil {
		return
	}

	invite, err := state.Invite(acceptPubKeyBytes)
	if err != nil {
		return
	}

	encPrivKey, signPrivKey := state.UserKeys()
	var symKeys [][]byte
	for _, rk := range invite.EncryptedReadKeys {
		dec, err := encSymKey.Decrypt(rk)
		if err != nil {
			return nil, err
		}
		newEnc, err := encPrivKey.GetPublic().Encrypt(dec)
		if err != nil {
			return nil, err
		}
		symKeys = append(symKeys, newEnc)
	}
	idSignature, err := acceptPrivKey.Sign(state.Identity())
	if err != nil {
		return
	}
	encPubKeyBytes, err := encPrivKey.GetPublic().Raw()
	if err != nil {
		return
	}

	userJoin := &aclrecordproto.ACLUserJoin{
		Identity:          state.Identity(),
		EncryptionKey:     encPubKeyBytes,
		AcceptSignature:   idSignature,
		AcceptPubKey:      acceptPubKeyBytes,
		EncryptedReadKeys: symKeys,
	}
	aclData := &aclrecordproto.ACLData{AclContent: []*aclrecordproto.ACLContentValue{
		{Value: &aclrecordproto.ACLContentValue_UserJoin{UserJoin: userJoin}},
	}}
	marshalledJoin, err := aclData.Marshal()
	if err != nil {
		return
	}
	aclRecord := &aclrecordproto.ACLRecord{
		PrevId:             state.LastRecordId(),
		Identity:           state.Identity(),
		Data:               marshalledJoin,
		CurrentReadKeyHash: state.CurrentReadKeyHash(),
		Timestamp:          time.Now().UnixNano(),
	}
	marshalledRecord, err := aclRecord.Marshal()
	if err != nil {
		return
	}
	recSignature, err := signPrivKey.Sign(marshalledRecord)
	if err != nil {
		return
	}
	rec = &aclrecordproto.RawACLRecord{
		Payload:   marshalledRecord,
		Signature: recSignature,
	}
	return
}

func (a *aclRecordBuilder) ConvertFromRaw(rawIdRecord *aclrecordproto.RawACLRecordWithId) (rec *ACLRecord, err error) {
	rawRec := &aclrecordproto.RawACLRecord{}
	err = proto.Unmarshal(rawIdRecord.Payload, rawRec)
	if err != nil {
		return
	}

	if rawIdRecord.Id == a.id {
		aclRoot := &aclrecordproto.ACLRoot{}
		err = proto.Unmarshal(rawRec.Payload, aclRoot)
		if err != nil {
			return
		}

		rec = &ACLRecord{
			Id:                 rawIdRecord.Id,
			CurrentReadKeyHash: aclRoot.CurrentReadKeyHash,
			Timestamp:          aclRoot.Timestamp,
			Signature:          rawRec.Signature,
			Identity:           aclRoot.Identity,
			Model:              aclRoot,
		}
	} else {
		aclRecord := &aclrecordproto.ACLRecord{}
		err = proto.Unmarshal(rawRec.Payload, aclRecord)
		if err != nil {
			return
		}

		rec = &ACLRecord{
			Id:                 rawIdRecord.Id,
			PrevId:             aclRecord.PrevId,
			CurrentReadKeyHash: aclRecord.CurrentReadKeyHash,
			Timestamp:          aclRecord.Timestamp,
			Data:               aclRecord.Data,
			Signature:          rawRec.Signature,
			Identity:           aclRecord.Identity,
		}
	}

	err = verifyRaw(a.keychain, rawRec, rawIdRecord, rec.Identity)
	return
}

func verifyRaw(
	keychain *common.Keychain,
	rawRec *aclrecordproto.RawACLRecord,
	recWithId *aclrecordproto.RawACLRecordWithId,
	identity []byte) (err error) {
	identityKey, err := keychain.GetOrAdd(string(identity))
	if err != nil {
		return
	}

	// verifying signature
	res, err := identityKey.Verify(rawRec.Payload, rawRec.Signature)
	if err != nil {
		return
	}
	if !res {
		err = ErrInvalidSignature
		return
	}

	// verifying ID
	if !cid.VerifyCID(recWithId.Payload, recWithId.Id) {
		err = ErrIncorrectCID
	}
	return
}
