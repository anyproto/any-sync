package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type ListStorage interface {
	Storage
	Head() (*aclpb.RawRecord, error)

	GetRawRecord(ctx context.Context, id string) (*aclpb.RawRecord, error)
	AddRawRecord(ctx context.Context, rec *aclpb.RawRecord) error
}
