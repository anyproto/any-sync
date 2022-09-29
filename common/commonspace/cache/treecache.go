package cache

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
)

const CName = "commonspace.cache"

var ErrSpaceNotFound = errors.New("space not found")

type TreeContainer interface {
	Tree() tree.ObjectTree
}

type TreeResult struct {
	Release       func()
	TreeContainer TreeContainer
}

type BuildFunc = func(ctx context.Context, id string, listener updatelistener.UpdateListener) (tree.ObjectTree, error)

type TreeCache interface {
	app.ComponentRunnable
	GetTree(ctx context.Context, spaceId, treeId string) (TreeResult, error)
}
