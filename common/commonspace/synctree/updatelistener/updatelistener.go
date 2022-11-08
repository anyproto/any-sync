//go:generate mockgen -destination mock_updatelistener/mock_updatelistener.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener UpdateListener
package updatelistener

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
)

type UpdateListener interface {
	Update(tree tree.ObjectTree)
	Rebuild(tree tree.ObjectTree)
}
