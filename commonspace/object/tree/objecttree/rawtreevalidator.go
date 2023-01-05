package objecttree

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
)

func ValidateRawTree(payload treestorage.TreeStorageCreatePayload, aclList list.AclList) (err error) {
	treeStorage, err := treestorage.NewInMemoryTreeStorage(payload.RootRawChange, payload.Heads, payload.Changes)
	if err != nil {
		return
	}

	_, err = BuildObjectTree(treeStorage, aclList)
	return
}
