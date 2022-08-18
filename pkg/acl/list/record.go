package list

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"

type Record struct {
	Id          string
	Content     *aclpb.Record
	ParsedModel interface{}
	Sign        []byte
}

func NewRecord(id string, aclRecord *aclpb.Record) *Record {
	return &Record{
		Id:      id,
		Content: aclRecord,
	}
}
