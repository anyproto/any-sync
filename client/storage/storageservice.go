package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/dgraph-io/badger/v3"
)

type storageService struct {
	db *badger.DB
}

func New() storage.SpaceStorageProvider {
	return &storageService{}
}

func (s *storageService) Init(a *app.App) (err error) {
	provider := a.MustComponent(badgerprovider.CName).(badgerprovider.BadgerProvider)
	s.db = provider.Badger()
	return
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
