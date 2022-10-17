package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cid"
	"github.com/gogo/protobuf/proto"
)

type ACLRecordBuilder interface {
	ConvertFromRaw(rawIdRecord *aclrecordproto.RawACLRecordWithId) (rec *ACLRecord, err error)
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
