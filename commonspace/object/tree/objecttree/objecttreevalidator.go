package objecttree

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/slice"
)

type TreeStorageCreator interface {
	CreateTreeStorage(payload treestorage.TreeStorageCreatePayload) (treestorage.TreeStorage, error)
}

type InMemoryStorageCreator struct{}

func (i InMemoryStorageCreator) CreateTreeStorage(payload treestorage.TreeStorageCreatePayload) (treestorage.TreeStorage, error) {
	return treestorage.NewInMemoryTreeStorage(payload.RootRawChange, payload.Heads, payload.Changes)
}

type ValidatorFunc func(payload treestorage.TreeStorageCreatePayload, storageCreator TreeStorageCreator, aclList list.AclList) (ret ObjectTree, err error)

type ObjectTreeValidator interface {
	// ValidateFullTree should always be entered while holding a read lock on AclList
	ValidateFullTree(tree *Tree, aclList list.AclList) error
	// ValidateNewChanges should always be entered while holding a read lock on AclList
	ValidateNewChanges(tree *Tree, aclList list.AclList, newChanges []*Change) error
	FilterChanges(aclList list.AclList, changes []*Change, snapshots []*Change, indexes []int) (filteredHeads bool, filtered, filteredSnapshots []*Change, newIndexes []int)
}

type noOpTreeValidator struct {
	filterFunc func(ch *Change) bool
	fail       bool
}

func (n *noOpTreeValidator) ValidateFullTree(tree *Tree, aclList list.AclList) error {
	if n.fail {
		return fmt.Errorf("failed")
	}
	return nil
}

func (n *noOpTreeValidator) ValidateNewChanges(tree *Tree, aclList list.AclList, newChanges []*Change) error {
	if n.fail {
		return fmt.Errorf("failed")
	}
	return nil
}

func (n *noOpTreeValidator) FilterChanges(aclList list.AclList, changes []*Change, snapshots []*Change, indexes []int) (filteredHeads bool, filtered, filteredSnapshots []*Change, newIndexes []int) {
	if n.filterFunc == nil {
		return false, changes, snapshots, indexes
	}
	for idx, c := range changes {
		// only taking changes which we can read
		if n.filterFunc(c) {
			newIndexes = append(newIndexes, indexes[idx])
			filtered = append(filtered, c)
			if c.IsSnapshot {
				filteredSnapshots = append(filteredSnapshots, c)
			}
		} else {
			filteredHeads = true
		}
	}
	return
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
	tree.IterateSkip(tree.RootId(), func(c *Change) (isContinue bool) {
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

func (v *objectTreeValidator) FilterChanges(aclList list.AclList, changes []*Change, snapshots []*Change, indexes []int) (filteredHeads bool, filtered, filteredSnapshots []*Change, newIndexes []int) {
	if !v.shouldFilter {
		return false, changes, snapshots, indexes
	}
	aclList.RLock()
	defer aclList.RUnlock()
	state := aclList.AclState()
	for idx, c := range changes {
		// this has to be a root
		if c.PreviousIds == nil {
			newIndexes = append(newIndexes, indexes[idx])
			filtered = append(filtered, c)
			filteredSnapshots = append(filteredSnapshots, c)
			continue
		}
		// only taking changes which we can read and for which we have acl heads
		if keys, exists := state.Keys()[c.ReadKeyId]; aclList.HasHead(c.AclHeadId) && exists && keys.ReadKey != nil {
			newIndexes = append(newIndexes, indexes[idx])
			filtered = append(filtered, c)
			if c.IsSnapshot {
				filteredSnapshots = append(filteredSnapshots, c)
			}
			continue
		}
		// if we filtered at least one change this can be the change between heads and other changes
		// thus we cannot use heads
		filteredHeads = true
	}
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

func ValidateRawTreeDefault(payload treestorage.TreeStorageCreatePayload, storageCreator TreeStorageCreator, aclList list.AclList) (objTree ObjectTree, err error) {
	treeStorage, err := storageCreator.CreateTreeStorage(treestorage.TreeStorageCreatePayload{
		RootRawChange: payload.RootRawChange,
		Heads:         []string{payload.RootRawChange.Id},
	})
	if err != nil {
		return
	}
	tree, err := BuildEmptyDataObjectTree(treeStorage, aclList)
	if err != nil {
		return
	}
	tree.Lock()
	defer tree.Unlock()
	res, err := tree.AddRawChanges(context.Background(), RawChangesPayload{
		NewHeads:   payload.Heads,
		RawChanges: payload.Changes,
	})
	if err != nil {
		return
	}
	if !slice.UnsortedEquals(res.Heads, payload.Heads) {
		return nil, fmt.Errorf("heads mismatch: %v != %v, %w", res.Heads, payload.Heads, ErrHasInvalidChanges)
	}
	// if tree has only one change we still should check if the snapshot id is same as root
	if IsEmptyDerivedTree(tree) {
		return nil, ErrDerived
	}
	return tree, nil
}

func ValidateFilterRawTree(payload treestorage.TreeStorageCreatePayload, storageCreator TreeStorageCreator, aclList list.AclList) (objTree ObjectTree, err error) {
	aclList.RLock()
	if !aclList.AclState().HadReadPermissions(aclList.AclState().Identity()) {
		aclList.RUnlock()
		return nil, list.ErrNoReadKey
	}
	aclList.RUnlock()
	treeStorage, err := storageCreator.CreateTreeStorage(treestorage.TreeStorageCreatePayload{
		RootRawChange: payload.RootRawChange,
		Heads:         []string{payload.RootRawChange.Id},
	})
	if err != nil {
		return
	}
	tree, err := BuildEmptyDataKeyFilterableObjectTree(treeStorage, aclList)
	if err != nil {
		return
	}
	tree.Lock()
	defer tree.Unlock()
	_, err = tree.AddRawChanges(context.Background(), RawChangesPayload{
		NewHeads:   payload.Heads,
		RawChanges: payload.Changes,
	})
	if err != nil {
		return
	}
	if IsEmptyTree(tree) {
		return nil, ErrNoChangeInTree
	}
	return tree, nil
}

func ValidateRawTree(payload treestorage.TreeStorageCreatePayload, aclList list.AclList) (err error) {
	_, err = ValidateRawTreeDefault(payload, InMemoryStorageCreator{}, aclList)
	return
}
