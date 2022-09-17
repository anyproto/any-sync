package cache

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
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

type TreeCache interface {
	app.ComponentRunnable
	GetTree(ctx context.Context, id string) (TreeResult, error)
}
