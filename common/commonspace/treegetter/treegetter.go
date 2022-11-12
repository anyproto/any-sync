//go:generate mockgen -destination mock_treegetter/mock_treegetter.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter TreeGetter
package treegetter

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
)

const CName = "commonspace.treeGetter"

var ErrSpaceNotFound = errors.New("space not found")

type TreeGetter interface {
	app.ComponentRunnable
	GetTree(ctx context.Context, spaceId, treeId string) (tree.ObjectTree, error)
	DeleteTree(ctx context.Context, spaceId, treeId string) error
}
