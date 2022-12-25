package objecttree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list"
)

type ObjectTreeValidator interface {
	// ValidateFullTree should always be entered while holding a read lock on ACLList
	ValidateFullTree(tree *Tree, aclList list.ACLList) error
	// ValidateNewChanges should always be entered while holding a read lock on ACLList
	ValidateNewChanges(tree *Tree, aclList list.ACLList, newChanges []*Change) error
}

type objectTreeValidator struct{}

func newTreeValidator() ObjectTreeValidator {
	return &objectTreeValidator{}
}

func (v *objectTreeValidator) ValidateFullTree(tree *Tree, aclList list.ACLList) (err error) {
	tree.Iterate(tree.RootId(), func(c *Change) (isContinue bool) {
		err = v.validateChange(tree, aclList, c)
		return err == nil
	})
	return err
}

func (v *objectTreeValidator) ValidateNewChanges(tree *Tree, aclList list.ACLList, newChanges []*Change) (err error) {
	for _, c := range newChanges {
		err = v.validateChange(tree, aclList, c)
		if err != nil {
			return
		}
	}
	return
}

func (v *objectTreeValidator) validateChange(tree *Tree, aclList list.ACLList, c *Change) (err error) {
	var (
		perm  list.UserPermissionPair
		state = aclList.ACLState()
	)
	// checking if the user could write
	perm, err = state.PermissionsAtRecord(c.AclHeadId, c.Identity)
	if err != nil {
		return
	}

	if perm.Permission != aclrecordproto.ACLUserPermissions_Writer && perm.Permission != aclrecordproto.ACLUserPermissions_Admin {
		err = list.ErrInsufficientPermissions
		return
	}

	if c.Id == tree.RootId() {
		return
	}

	// checking if the change refers to later acl heads than its previous ids
	for _, id := range c.PreviousIds {
		prevChange := tree.attached[id]
		if prevChange.AclHeadId == c.AclHeadId {
			continue
		}
		var after bool
		after, err = aclList.IsAfter(c.AclHeadId, prevChange.AclHeadId)
		if err != nil {
			return
		}
		if !after {
			err = fmt.Errorf("current acl head id (%s) should be after each of the previous ones (%s)", c.AclHeadId, prevChange.AclHeadId)
			return
		}
	}
	return
}
