package spacestorage

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"sync"
)

type InMemorySpaceStorage struct {
	id              string
	isDeleted       bool
	spaceSettingsId string
	treeDeleted     map[string]string
	trees           map[string]treestorage.TreeStorage
	aclStorage      liststorage.ListStorage
	spaceHeader     *spacesyncproto.RawSpaceHeaderWithId
	spaceHash       string
	sync.Mutex
}

func (i *InMemorySpaceStorage) Init(a *app.App) (err error) {
	return nil
}

func (i *InMemorySpaceStorage) Name() (name string) {
	return StorageName
}

func NewInMemorySpaceStorage(payload SpaceStorageCreatePayload) (SpaceStorage, error) {
	aclStorage, err := liststorage.NewInMemoryAclListStorage(payload.AclWithId.Id, []*aclrecordproto.RawAclRecordWithId{payload.AclWithId})
	if err != nil {
		return nil, err
	}
	inMemory := &InMemorySpaceStorage{
		id:              payload.SpaceHeaderWithId.Id,
		spaceSettingsId: payload.SpaceSettingsWithId.Id,
		treeDeleted:     map[string]string{},
		trees:           map[string]treestorage.TreeStorage{},
		aclStorage:      aclStorage,
		spaceHeader:     payload.SpaceHeaderWithId,
	}
	_, err = inMemory.CreateTreeStorage(treestorage.TreeStorageCreatePayload{
		RootRawChange: payload.SpaceSettingsWithId,
		Changes:       []*treechangeproto.RawTreeChangeWithId{payload.SpaceSettingsWithId},
		Heads:         []string{payload.SpaceSettingsWithId.Id},
	})
	if err != nil {
		return nil, err
	}
	return inMemory, nil
}

func (i *InMemorySpaceStorage) Id() string {
	return i.id
}

func (i *InMemorySpaceStorage) SetSpaceDeleted() error {
	i.Lock()
	defer i.Unlock()
	i.isDeleted = true
	return nil
}

func (i *InMemorySpaceStorage) IsSpaceDeleted() (bool, error) {
	i.Lock()
	defer i.Unlock()
	return i.isDeleted, nil
}

func (i *InMemorySpaceStorage) SetTreeDeletedStatus(id, state string) error {
	i.Lock()
	defer i.Unlock()
	i.treeDeleted[id] = state
	return nil
}

func (i *InMemorySpaceStorage) TreeDeletedStatus(id string) (string, error) {
	i.Lock()
	defer i.Unlock()
	return i.treeDeleted[id], nil
}

func (i *InMemorySpaceStorage) SpaceSettingsId() string {
	return i.spaceSettingsId
}

func (i *InMemorySpaceStorage) AclStorage() (liststorage.ListStorage, error) {
	return i.aclStorage, nil
}

func (i *InMemorySpaceStorage) SpaceHeader() (*spacesyncproto.RawSpaceHeaderWithId, error) {
	return i.spaceHeader, nil
}

func (i *InMemorySpaceStorage) StoredIds() ([]string, error) {
	i.Lock()
	defer i.Unlock()
	var allIds []string
	for id := range i.trees {
		allIds = append(allIds, id)
	}
	return allIds, nil
}

func (i *InMemorySpaceStorage) TreeRoot(id string) (*treechangeproto.RawTreeChangeWithId, error) {
	i.Lock()
	defer i.Unlock()
	treeStorage, exists := i.trees[id]
	if !exists {
		return nil, treestorage.ErrUnknownTreeId
	}
	return treeStorage.Root()
}

func (i *InMemorySpaceStorage) TreeStorage(id string) (treestorage.TreeStorage, error) {
	i.Lock()
	defer i.Unlock()
	treeStorage, exists := i.trees[id]
	if !exists {
		return nil, treestorage.ErrUnknownTreeId
	}
	return treeStorage, nil
}

func (i *InMemorySpaceStorage) HasTree(id string) (bool, error) {
	i.Lock()
	defer i.Unlock()
	_, exists := i.trees[id]
	return exists, nil
}

func (i *InMemorySpaceStorage) CreateTreeStorage(payload treestorage.TreeStorageCreatePayload) (treestorage.TreeStorage, error) {
	i.Lock()
	defer i.Unlock()
	storage, err := treestorage.NewInMemoryTreeStorage(payload.RootRawChange, payload.Heads, payload.Changes)
	if err != nil {
		return nil, err
	}
	i.trees[payload.RootRawChange.Id] = storage
	return storage, nil
}

func (i *InMemorySpaceStorage) WriteSpaceHash(hash string) error {
	i.Lock()
	defer i.Unlock()
	i.spaceHash = hash
	return nil
}

func (i *InMemorySpaceStorage) ReadSpaceHash() (hash string, err error) {
	i.Lock()
	defer i.Unlock()
	return i.spaceHash, nil
}

func (i *InMemorySpaceStorage) Close() error {
	return nil
}

func (i *InMemorySpaceStorage) AllTrees() map[string]treestorage.TreeStorage {
	i.Lock()
	defer i.Unlock()
	cp := map[string]treestorage.TreeStorage{}
	for id, store := range i.trees {
		cp[id] = store
	}
	return cp
}

func (i *InMemorySpaceStorage) SetTrees(trees map[string]treestorage.TreeStorage) {
	i.Lock()
	defer i.Unlock()
	i.trees = trees
}

func (i *InMemorySpaceStorage) CopyStorage() *InMemorySpaceStorage {
	i.Lock()
	defer i.Unlock()
	copyTreeDeleted := map[string]string{}
	for id, status := range i.treeDeleted {
		copyTreeDeleted[id] = status
	}
	copyTrees := map[string]treestorage.TreeStorage{}
	for id, store := range i.trees {
		copyTrees[id] = store
	}
	return &InMemorySpaceStorage{
		id:              i.id,
		isDeleted:       i.isDeleted,
		spaceSettingsId: i.spaceSettingsId,
		treeDeleted:     copyTreeDeleted,
		trees:           copyTrees,
		aclStorage:      i.aclStorage,
		spaceHeader:     i.spaceHeader,
		spaceHash:       i.spaceHash,
		Mutex:           sync.Mutex{},
	}
}
