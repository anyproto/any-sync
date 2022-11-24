package commonspace

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	treestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
)

type commonStorage struct {
	storage.SpaceStorage
}

func newCommonStorage(spaceStorage storage.SpaceStorage) storage.SpaceStorage {
	return &commonStorage{
		SpaceStorage: spaceStorage,
	}
}

func (c *commonStorage) CreateTreeStorage(payload treestorage.TreeStorageCreatePayload) (store treestorage.TreeStorage, err error) {
	status, err := c.TreeDeletedStatus(payload.RootRawChange.Id)
	if err != nil {
		return
	}
	if status == "" {
		return c.CreateTreeStorage(payload)
	}
	err = storage.ErrTreeStorageAlreadyDeleted
	return
}
