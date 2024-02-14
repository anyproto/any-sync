package objecttree

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/slice"
)

type ValidatorFunc func(payload treestorage.TreeStorageCreatePayload, buildFunc BuildObjectTreeFunc, aclList list.AclList) (retPayload treestorage.TreeStorageCreatePayload, err error)

type ObjectTreeValidator interface {
	// ValidateFullTree should always be entered while holding a read lock on AclList
	ValidateFullTree(tree *Tree, aclList list.AclList) error
	// ValidateNewChanges should always be entered while holding a read lock on AclList
	ValidateNewChanges(tree *Tree, aclList list.AclList, newChanges []*Change) error
	FilterChanges(aclList list.AclList, heads []string, changes []*Change, snapshots []*Change, indexes []int) (filteredHeads bool, filtered, filteredSnapshots []*Change, newIndexes []int)
}

type noOpTreeValidator struct{}

func (n *noOpTreeValidator) ValidateFullTree(tree *Tree, aclList list.AclList) error {
	return nil
}

func (n *noOpTreeValidator) ValidateNewChanges(tree *Tree, aclList list.AclList, newChanges []*Change) error {
	return nil
}

func (n *noOpTreeValidator) FilterChanges(aclList list.AclList, heads []string, changes []*Change, snapshots []*Change, indexes []int) (filteredHeads bool, filtered, filteredSnapshots []*Change, newIndexes []int) {
	return false, changes, snapshots, indexes
}

type objectTreeValidator struct {
	validateKeys bool
	shouldFilter bool
}

func newTreeValidator(validateKeys bool, filterChanges bool) ObjectTreeValidator {
	return &objectTreeValidator{
		validateKeys: validateKeys,
		shouldFilter: filterChanges,
	}
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

func (v *objectTreeValidator) FilterChanges(aclList list.AclList, heads []string, changes []*Change, snapshots []*Change, indexes []int) (filteredHeads bool, filtered, filteredSnapshots []*Change, newIndexes []int) {
	if !v.shouldFilter {
		return false, changes, snapshots, indexes
	}
	aclList.RLock()
	defer aclList.RUnlock()
	state := aclList.AclState()
	var existingHeadsCount int
	for idx, c := range changes {
		// only taking changes which we can read
		if _, exists := state.Keys()[c.ReadKeyId]; exists {
			if slice.FindPos(heads, c.Id) != -1 {
				existingHeadsCount++
			}
			newIndexes = append(newIndexes, indexes[idx])
			filtered = append(filtered, c)
			if c.IsSnapshot {
				filteredSnapshots = append(filteredSnapshots, c)
			}
		}
	}
	filteredHeads = existingHeadsCount != len(heads)
	return
}

func (v *objectTreeValidator) validateChange(tree *Tree, aclList list.AclList, c *Change) (err error) {
	var (
		perms list.AclPermissions
		state = aclList.AclState()
	)
	if c.IsDerived {
		return nil
	}
	// checking if the user could write
	perms, err = state.PermissionsAtRecord(c.AclHeadId, c.Identity)
	if err != nil {
		return
	}
	if !perms.CanWrite() {
		err = list.ErrInsufficientPermissions
		return
	}
	if c.Id == tree.RootId() {
		return
	}
	if v.validateKeys {
		keys, exists := state.Keys()[c.ReadKeyId]
		if !exists || keys.ReadKey == nil {
			return list.ErrNoReadKey
		}
	}

	// checking if the change refers to later acl heads than its previous ids
	for _, id := range c.PreviousIds {
		prevChange := tree.attached[id]
		if prevChange.AclHeadId == c.AclHeadId {
			continue
		}
		if prevChange.IsDerived {
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

func ValidateRawTreeBuildFunc(payload treestorage.TreeStorageCreatePayload, buildFunc BuildObjectTreeFunc, aclList list.AclList) (newPayload treestorage.TreeStorageCreatePayload, err error) {
	treeStorage, err := treestorage.NewInMemoryTreeStorage(payload.RootRawChange, []string{payload.RootRawChange.Id}, nil)
	if err != nil {
		return
	}
	tree, err := buildFunc(treeStorage, aclList)
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
		return payload, ErrHasInvalidChanges
	}
	// if tree has only one change we still should check if the snapshot id is same as root
	if IsEmptyDerivedTree(tree) {
		return payload, ErrDerived
	}
	return payload, nil
}

func ValidateFilterReadKeyRawTreeBuildFunc(payload treestorage.TreeStorageCreatePayload, buildFunc BuildObjectTreeFunc, aclList list.AclList) (retPayload treestorage.TreeStorageCreatePayload, err error) {
	aclList.RLock()
	if !aclList.AclState().HadReadPermissions(aclList.AclState().Identity()) {
		aclList.RUnlock()
		return payload, list.ErrNoReadKey
	}
	aclList.RUnlock()
	treeStorage, err := treestorage.NewInMemoryTreeStorage(payload.RootRawChange, []string{payload.RootRawChange.Id}, nil)
	if err != nil {
		return
	}
	tree, err := BuildKeyFilterableObjectTree(treeStorage, aclList)
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
	return treestorage.TreeStorageCreatePayload{
		RootRawChange: payload.RootRawChange,
		Heads:         res.Heads,
		Changes:       treeStorage.(*treestorage.InMemoryTreeStorage).AllChanges(),
	}, nil
}

func ValidateRawTree(payload treestorage.TreeStorageCreatePayload, aclList list.AclList) (err error) {
	_, err = ValidateRawTreeBuildFunc(payload, BuildObjectTree, aclList)
	return
}
