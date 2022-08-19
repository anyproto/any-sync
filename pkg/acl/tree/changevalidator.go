package tree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
)

type DocTreeValidator interface {
	ValidateTree(tree *Tree, aclList list.ACLList) error
}

type docTreeValidator struct{}

func newTreeValidator() DocTreeValidator {
	return &docTreeValidator{}
}

func (v *docTreeValidator) ValidateTree(tree *Tree, aclList list.ACLList) (err error) {
	// TODO: add validation logic where we check that the change refers to correct acl heads
	//  that means that more recent changes should refer to more recent acl heads
	var (
		perm  list.UserPermissionPair
		state = aclList.ACLState()
	)

	tree.Iterate(tree.RootId(), func(c *Change) (isContinue bool) {
		// checking if the user could write
		perm, err = state.PermissionsAtRecord(c.Id, c.Content.Identity)
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
