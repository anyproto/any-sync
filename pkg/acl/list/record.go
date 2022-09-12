package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/gogo/protobuf/proto"
)

type ACLRecord struct {
	Id       string
	Content  *aclpb.ACLRecord
	Identity string
	Model    interface{}
	Sign     []byte
}

func NewRecord(id string, aclRecord *aclpb.ACLRecord) *ACLRecord {
	return &ACLRecord{
		Id:       id,
		Content:  aclRecord,
		Identity: string(aclRecord.Identity),
	}
}

func NewFromRawRecord(rawRec *aclpb.RawACLRecord) (*ACLRecord, error) {
	aclRec := &aclpb.ACLRecord{}
	err := proto.Unmarshal(rawRec.Payload, aclRec)
	if err != nil {
		return nil, err
	}

	return &ACLRecord{
		Id:       rawRec.Id,
		Content:  aclRec,
		Sign:     rawRec.Signature,
		Identity: string(aclRec.Identity),
	}, nil
}
