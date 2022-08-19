package list

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type Storage interface {
	ID() string
	Head() (*aclpb.RawRecord, error)
	Header() (*aclpb.Header, error)
	GetRecord(ctx context.Context, id string) (*aclpb.RawRecord, error)
	AddRecord(ctx context.Context, rec *aclpb.RawRecord) error
}
