//go:generate mockgen -destination mock_treegetter/mock_treegetter.go github.com/anytypeio/any-sync/commonspace/object/treegetter TreeGetter
package treegetter

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
)

const CName = "common.object.treegetter"

type TreeGetter interface {
	app.ComponentRunnable
	GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error)
	DeleteTree(ctx context.Context, spaceId, treeId string) error
}
