package tree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
)

type ObjectTreeValidator interface {
	// ValidateTree should always be entered while holding a read lock on ACLList
	ValidateTree(tree *Tree, aclList list.ACLList) error
}

type objectTreeValidator struct{}

func newTreeValidator() ObjectTreeValidator {
	return &objectTreeValidator{}
}

func (v *objectTreeValidator) ValidateTree(tree *Tree, aclList list.ACLList) (err error) {

	var (
		perm  list.UserPermissionPair
		state = aclList.ACLState()
	)

	tree.Iterate(tree.RootId(), func(c *Change) (isContinue bool) {
		// checking if the user could write
		perm, err = state.PermissionsAtRecord(c.Content.AclHeadId, c.Content.Identity)
		if err != nil {
			return false
		}

		if perm.Permission != aclpb.ACLChange_Writer && perm.Permission != aclpb.ACLChange_Admin {
			err = list.ErrInsufficientPermissions
			return false
		}

		// checking if the change refers to later acl heads than its previous ids
		for _, id := range c.PreviousIds {
			prevChange := tree.attached[id]
			if prevChange.Content.AclHeadId == c.Content.AclHeadId {
				continue
			}
			var after bool
			after, err = aclList.IsAfter(c.Content.AclHeadId, prevChange.Content.AclHeadId)
			if err != nil {
				return false
			}
			if !after {
				err = fmt.Errorf("current acl head id (%s) should be after each of the previous ones (%s)", c.Content.AclHeadId, prevChange.Content.AclHeadId)
				return false
			}
		}
		return true
	})
	return err
}
