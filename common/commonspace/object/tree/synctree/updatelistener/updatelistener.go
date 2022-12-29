//go:generate mockgen -destination mock_updatelistener/mock_updatelistener.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/updatelistener UpdateListener
package updatelistener

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree"
)

type UpdateListener interface {
	Update(tree objecttree.ObjectTree)
	Rebuild(tree objecttree.ObjectTree)
}
