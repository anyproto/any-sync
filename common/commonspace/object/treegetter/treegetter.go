//go:generate mockgen -destination mock_treegetter/mock_treegetter.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/treegetter TreeGetter
package treegetter

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree"
)

const CName = "common.object.treegetter"

type TreeGetter interface {
	app.ComponentRunnable
	GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error)
	DeleteTree(ctx context.Context, spaceId, treeId string) error
}
