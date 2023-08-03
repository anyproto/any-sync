package objecttree

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/slice"
)

type ObjectTreeValidator interface {
	// ValidateFullTree should always be entered while holding a read lock on AclList
	ValidateFullTree(tree *Tree, aclList list.AclList) error
	// ValidateNewChanges should always be entered while holding a read lock on AclList
	ValidateNewChanges(tree *Tree, aclList list.AclList, newChanges []*Change) error
}

type noOpTreeValidator struct{}

func (n *noOpTreeValidator) ValidateFullTree(tree *Tree, aclList list.AclList) error {
	return nil
}

func (n *noOpTreeValidator) ValidateNewChanges(tree *Tree, aclList list.AclList, newChanges []*Change) error {
	return nil
}

type objectTreeValidator struct{}

func newTreeValidator() ObjectTreeValidator {
	return &objectTreeValidator{}
}

func (v *objectTreeValidator) ValidateFullTree(tree *Tree, aclList list.AclList) (err error) {
	tree.Iterate(tree.RootId(), func(c *Change) (isContinue bool) {
		err = v.validateChange(tree, aclList, c)
		return err == nil
	})
	return err
}

func (v *objectTreeValidator) ValidateNewChanges(tree *Tree, aclList list.AclList, newChanges []*Change) (err error) {
	for _, c := range newChanges {
		err = v.validateChange(tree, aclList, c)
		if err != nil {
			return
		}
	}
	return
}

func (v *objectTreeValidator) validateChange(tree *Tree, aclList list.AclList, c *Change) (err error) {
	var (
		userState list.AclAccountState
		state     = aclList.AclState()
	)
	// checking if the user could write
	userState, err = state.StateAtRecord(c.AclHeadId, c.Identity)
	if err != nil {
		return
	}
	if !userState.Permissions.CanWrite() {
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

func ValidateRawTree(payload treestorage.TreeStorageCreatePayload, aclList list.AclList) (err error) {
	treeStorage, err := treestorage.NewInMemoryTreeStorage(payload.RootRawChange, []string{payload.RootRawChange.Id}, nil)
	if err != nil {
		return
	}
	tree, err := BuildObjectTree(treeStorage, aclList)
	if err != nil {
		return
	}
	res, err := tree.AddRawChanges(context.Background(), RawChangesPayload{
		NewHeads:   payload.Heads,
		RawChanges: payload.Changes,
	})
	if err != nil {
		return
	}
	if !slice.UnsortedEquals(res.Heads, payload.Heads) {
		return ErrHasInvalidChanges
	}
	return
}
