package objecttree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
)

func ValidateRawTree(payload treestorage.TreeStorageCreatePayload, aclList list.AclList) (err error) {
	provider := treestorage.NewInMemoryTreeStorageProvider()
	treeStorage, err := provider.CreateTreeStorage(payload)
	if err != nil {
		return
	}

	_, err = BuildObjectTree(treeStorage, aclList)
	return
}
