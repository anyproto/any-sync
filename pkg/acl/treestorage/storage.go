package treestorage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type TreeStorage interface {
	ID() (string, error)

	Header() (*aclpb.Header, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error

	AddRawChange(change *aclpb.RawChange) error
	GetRawChange(ctx context.Context, recordID string) (*aclpb.RawChange, error)
}
