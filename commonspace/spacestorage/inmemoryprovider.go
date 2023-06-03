package spacestorage

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"sync"
)

func NewInMemorySpaceStorageProvider() SpaceStorageProvider {
	return &InMemorySpaceStorageProvider{
		storages: map[string]SpaceStorage{},
	}
}

type InMemorySpaceStorageProvider struct {
	storages map[string]SpaceStorage
	sync.Mutex
}

func (i *InMemorySpaceStorageProvider) Init(a *app.App) (err error) {
	return nil
}

func (i *InMemorySpaceStorageProvider) Name() (name string) {
	return CName
}

func (i *InMemorySpaceStorageProvider) WaitSpaceStorage(ctx context.Context, id string) (SpaceStorage, error) {
	i.Lock()
	defer i.Unlock()
	storage, exists := i.storages[id]
	if !exists {
		return nil, ErrSpaceStorageMissing
	}
	return storage, nil
}

func (i *InMemorySpaceStorageProvider) SpaceExists(id string) bool {
	i.Lock()
	defer i.Unlock()
	_, exists := i.storages[id]
	return exists
}

func (i *InMemorySpaceStorageProvider) CreateSpaceStorage(payload SpaceStorageCreatePayload) (SpaceStorage, error) {
	i.Lock()
	defer i.Unlock()
	spaceStorage, err := NewInMemorySpaceStorage(payload)
	if err != nil {
		return nil, err
	}
	i.storages[payload.SpaceHeaderWithId.Id] = spaceStorage
	return spaceStorage, nil
}

func (i *InMemorySpaceStorageProvider) SetStorage(storage SpaceStorage) {
	i.Lock()
	defer i.Unlock()
	i.storages[storage.Id()] = storage
}
