package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
)

type storageService struct {
	rootPath string
}

func New() storage.SpaceStorageProvider {
	return &storageService{}
}

func (s *storageService) Init(a *app.App) (err error) {
	cfg := a.MustComponent(config.CName).(*config.Config)
	s.rootPath = cfg.Storage.Path
	return nil
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
