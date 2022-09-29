package updatelistener

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"

type UpdateListener interface {
	Update(tree tree.ObjectTree)
	Rebuild(tree tree.ObjectTree)
}
