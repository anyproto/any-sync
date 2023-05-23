//go:generate mockgen -destination mock_updatelistener/mock_updatelistener.go github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener UpdateListener
package updatelistener

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
)

type UpdateListener interface {
	Update(tree objecttree.ObjectTree)
	Rebuild(tree objecttree.ObjectTree)
}
