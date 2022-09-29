package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
)

type ListStorage interface {
	Storage
	Root() (*aclrecordproto.RawACLRecordWithId, error)
	Head() (*aclrecordproto.RawACLRecordWithId, error)

	GetRawRecord(ctx context.Context, id string) (*aclrecordproto.RawACLRecordWithId, error)
	AddRawRecord(ctx context.Context, rec *aclrecordproto.RawACLRecordWithId) error
}
