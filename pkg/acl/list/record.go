package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/gogo/protobuf/proto"
)

type ACLRecord struct {
	Id       string
	Content  *aclrecordproto.ACLRecord
	Identity string
	Model    interface{}
	Sign     []byte
}

func NewRecord(id string, aclRecord *aclrecordproto.ACLRecord) *ACLRecord {
	return &ACLRecord{
		Id:       id,
		Content:  aclRecord,
		Identity: string(aclRecord.Identity),
	}
}

func NewFromRawRecord(rawRecWithId *aclrecordproto.RawACLRecordWithId) (aclRec *ACLRecord, err error) {
	rawRec := &aclrecordproto.RawACLRecord{}
	err = proto.Unmarshal(rawRecWithId.Payload, rawRec)
	if err != nil {
		return
	}

	protoAclRec := &aclrecordproto.ACLRecord{}
	err = proto.Unmarshal(rawRec.Payload, protoAclRec)
	if err != nil {
		return
	}

	return &ACLRecord{
		Id:       rawRecWithId.Id,
		Content:  protoAclRec,
		Sign:     rawRec.Signature,
		Identity: string(protoAclRec.Identity),
	}, nil
}

func NewFromRawRoot(rawRecWithId *aclrecordproto.RawACLRecordWithId) (aclRec *ACLRecord, err error) {
	rawRec := &aclrecordproto.RawACLRecord{}
	err = proto.Unmarshal(rawRecWithId.Payload, rawRec)
	if err != nil {
		return
	}

	protoAclRec := &aclrecordproto.ACLRecord{}
	err = proto.Unmarshal(rawRec.Payload, protoAclRec)
	if err != nil {
		return
	}

	return &ACLRecord{
		Id:       rawRecWithId.Id,
		Content:  protoAclRec,
		Sign:     rawRec.Signature,
		Identity: string(protoAclRec.Identity),
	}, nil
}

