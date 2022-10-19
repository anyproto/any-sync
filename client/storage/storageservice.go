package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/dgraph-io/badger/v3"
)

type storageService struct {
	keys storageServiceKeys
	db   *badger.DB
}

type ClientStorage interface {
	storage.SpaceStorageProvider
	AllSpaceIds() (ids []string, err error)
}

func New() ClientStorage {
	return &storageService{}
}

func (s *storageService) Init(a *app.App) (err error) {
	provider := a.MustComponent(badgerprovider.CName).(badgerprovider.BadgerProvider)
	s.db = provider.Badger()
	s.keys = newStorageServiceKeys()
	return
}

func (s *storageService) Name() (name string) {
	return storage.CName
}

func (s *storageService) SpaceStorage(id string) (storage.SpaceStorage, error) {
	return newSpaceStorage(s.db, id)
}

func (s *storageService) CreateSpaceStorage(payload storage.SpaceStorageCreatePayload) (storage.SpaceStorage, error) {
	return createSpaceStorage(s.db, payload)
}

func (s *storageService) AllSpaceIds() (ids []string, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = s.keys.SpacePrefix()

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			id := item.Key()
			if len(id) <= len(s.keys.SpacePrefix())+1 {
				continue
			}
			id = id[len(s.keys.SpacePrefix())+1:]
			ids = append(ids, string(id))
		}
		return nil
	})
	return
}
