package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"os"
)

type storageService struct {
	rootPath string
}

type NodeStorage interface {
	storage.SpaceStorageProvider
	AllSpaceIds() (ids []string, err error)
}

func New() NodeStorage {
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

func (s *storageService) AllSpaceIds() (ids []string, err error) {
	var files []string
	fileInfo, err := os.ReadDir(s.rootPath)
	if err != nil {
		return files, err
	}

	for _, file := range fileInfo {
		files = append(files, file.Name())
	}
	return files, nil
}
