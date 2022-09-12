package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type ListStorage interface {
	Storage
	Header() (*aclpb.ACLHeader, error)
	Head() (*aclpb.RawACLRecord, error)

	GetRawRecord(ctx context.Context, id string) (*aclpb.RawACLRecord, error)
	AddRawRecord(ctx context.Context, rec *aclpb.RawACLRecord) error
}
