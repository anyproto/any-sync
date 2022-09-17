package cache

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
)

const CName = "commonspace.cache"

type TreeContainer interface {
	Tree() tree.ObjectTree
}

type TreeResult struct {
	Release       func()
	TreeContainer TreeContainer
}

type BuildFunc = func(ctx context.Context, id string, listener synctree.UpdateListener) (tree.ObjectTree, error)

type TreeCache interface {
	GetTree(ctx context.Context, id string) (TreeResult, error)
	SetBuildFunc(f BuildFunc)

	Close() error
}
