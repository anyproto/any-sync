package commonspace

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
)

type commonStorage struct {
	spacestorage.SpaceStorage
}

func newCommonStorage(spaceStorage spacestorage.SpaceStorage) spacestorage.SpaceStorage {
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
		return c.SpaceStorage.CreateTreeStorage(payload)
	}
	err = spacestorage.ErrTreeStorageAlreadyDeleted
	return
}
