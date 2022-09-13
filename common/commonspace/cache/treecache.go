package cache

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
)

type TreeResult struct {
	Release func()
	Tree    tree.ObjectTree
}

type TreeCache interface {
	GetTree(ctx context.Context, id string) (TreeResult, error)
	AddTree(ctx context.Context, payload storage.TreeStorageCreatePayload) error
}
