package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
)

type storageService struct {
	rootPath string
}

func (s *storageService) Init(a *app.App) (err error) {
	//TODO implement me
	panic("implement me")
}

func (s *storageService) Name() (name string) {
	return storage.CName
}

func (s *storageService) SpaceStorage(id string) (storage.SpaceStorage, error) {
	return newSpaceStorage(s.rootPath, id)
}

func (s *storageService) CreateSpaceStorage(payload storage.SpaceStorageCreatePayload) (storage.SpaceStorage, error) {
	return createSpaceStorage(s.rootPath, payload)
}
