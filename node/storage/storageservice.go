package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage"
	"os"
)

type storageService struct {
	rootPath string
}

type NodeStorage interface {
	spacestorage.SpaceStorageProvider
	AllSpaceIds() (ids []string, err error)
}

func New() NodeStorage {
	return &storageService{}
}

func (s *storageService) Init(a *app.App) (err error) {
	cfg := a.MustComponent("config").(configGetter).GetStorage()
	s.rootPath = cfg.Path
	return nil
}

func (s *storageService) Name() (name string) {
	return spacestorage.CName
}

func (s *storageService) SpaceStorage(id string) (spacestorage.SpaceStorage, error) {
	return newSpaceStorage(s.rootPath, id)
}

func (s *storageService) CreateSpaceStorage(payload spacestorage.SpaceStorageCreatePayload) (spacestorage.SpaceStorage, error) {
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
