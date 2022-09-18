package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

func ValidateRawTree(payload storage.TreeStorageCreatePayload, aclList list.ACLList) (err error) {
	provider := storage.NewInMemoryTreeStorageProvider()
	treeStorage, err := provider.CreateTreeStorage(payload)
	if err != nil {
		return
	}

	_, err = BuildObjectTree(treeStorage, aclList)
	return
}
