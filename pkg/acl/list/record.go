package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/gogo/protobuf/proto"
)

type Record struct {
	Id      string
	Content *aclpb.Record
	Model   interface{}
	Sign    []byte
}

func NewRecord(id string, aclRecord *aclpb.Record) *Record {
	return &Record{
		Id:      id,
		Content: aclRecord,
	}
}

func NewFromRawRecord(rawRec *aclpb.RawRecord) (*Record, error) {
	aclRec := &aclpb.Record{}
	err := proto.Unmarshal(rawRec.Payload, aclRec)
	if err != nil {
		return nil, err
	}

	return &Record{
		Id:      rawRec.Id,
		Content: aclRec,
		Sign:    rawRec.Signature,
	}, nil
}
