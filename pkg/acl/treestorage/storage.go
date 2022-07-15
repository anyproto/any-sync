package treestorage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
)

type TreeStorage interface {
	TreeID() (string, error)

	Header() (*treepb.TreeHeader, error)
	Heads() ([]string, error)
	Orphans() ([]string, error)
	SetHeads(heads []string) error
	RemoveOrphans(orphan ...string) error
	AddOrphans(orphan ...string) error

	AddRawChange(change *aclpb.RawChange) error
	AddChange(change aclchanges.Change) error

	// TODO: have methods with raw changes also
	GetChange(ctx context.Context, recordID string) (*aclpb.RawChange, error)
}
