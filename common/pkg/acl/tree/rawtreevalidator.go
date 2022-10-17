package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	storage2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
)

func ValidateRawTree(payload storage2.TreeStorageCreatePayload, aclList list.ACLList) (err error) {
	provider := storage2.NewInMemoryTreeStorageProvider()
	treeStorage, err := provider.CreateTreeStorage(payload)
	if err != nil {
		return
	}

	_, err = BuildObjectTree(treeStorage, aclList)
	return
}
