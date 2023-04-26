//go:generate mockgen -destination mock_treemanager/mock_treemanager.go github.com/anytypeio/any-sync/commonspace/object/treemanager TreeManager
package treemanager

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
)

const CName = "common.object.treemanager"

type TreeManager interface {
	app.ComponentRunnable
	GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error)
	DeleteTree(ctx context.Context, spaceId, treeId string) error
}
