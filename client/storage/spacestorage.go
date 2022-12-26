package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/liststorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treechangeproto"
	storage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
	spacestorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/dgraph-io/badger/v3"
)

type spaceStorage struct {
	spaceId         string
	spaceSettingsId string
	objDb           *badger.DB
	keys            spaceKeys
	aclStorage      liststorage.ListStorage
	header          *spacesyncproto.RawSpaceHeaderWithId
}

var spaceValidationFunc = spacestorage.ValidateSpaceStorageCreatePayload

func newSpaceStorage(objDb *badger.DB, spaceId string) (store spacestorage.SpaceStorage, err error) {
	keys := newSpaceKeys(spaceId)
	err = objDb.View(func(txn *badger.Txn) error {
		header, err := getTxn(txn, keys.HeaderKey())
		if err != nil {
			return err
		}

		aclStorage, err := newListStorage(spaceId, objDb, txn)
		if err != nil {
			return err
		}

		spaceSettingsId, err := getTxn(txn, keys.SpaceSettingsId())
		if err != nil {
			return err
		}
		store = &spaceStorage{
			spaceId:         spaceId,
			spaceSettingsId: string(spaceSettingsId),
			objDb:           objDb,
			keys:            keys,
			header: &spacesyncproto.RawSpaceHeaderWithId{
				RawHeader: header,
				Id:        spaceId,
			},
			aclStorage: aclStorage,
		}
		return nil
	})
	if err == badger.ErrKeyNotFound {
		err = spacestorage.ErrSpaceStorageMissing
	}
	return
}

func createSpaceStorage(db *badger.DB, payload spacestorage.SpaceStorageCreatePayload) (store spacestorage.SpaceStorage, err error) {
	keys := newSpaceKeys(payload.SpaceHeaderWithId.Id)
	if hasDB(db, keys.HeaderKey()) {
		err = spacestorage.ErrSpaceStorageExists
		return
	}
	err = spaceValidationFunc(payload)
	if err != nil {
		return
	}

	spaceStore := &spaceStorage{
		spaceId:         payload.SpaceHeaderWithId.Id,
		objDb:           db,
		keys:            keys,
		spaceSettingsId: payload.SpaceSettingsWithId.Id,
		header:          payload.SpaceHeaderWithId,
	}
	_, err = spaceStore.CreateTreeStorage(storage.TreeStorageCreatePayload{
		RootRawChange: payload.SpaceSettingsWithId,
		Changes:       []*treechangeproto.RawTreeChangeWithId{payload.SpaceSettingsWithId},
		Heads:         []string{payload.SpaceSettingsWithId.Id},
	})
	if err != nil {
		return
	}
	err = db.Update(func(txn *badger.Txn) error {
		err = txn.Set(keys.SpaceSettingsId(), []byte(payload.SpaceSettingsWithId.Id))
		if err != nil {
			return err
		}
		aclStorage, err := createListStorage(payload.SpaceHeaderWithId.Id, db, txn, payload.AclWithId)
		if err != nil {
			return err
		}

		err = txn.Set(keys.HeaderKey(), payload.SpaceHeaderWithId.RawHeader)
		if err != nil {
			return err
		}

		spaceStore.aclStorage = aclStorage
		return nil
	})
	store = spaceStore
	return
}

func (s *spaceStorage) Id() string {
	return s.spaceId
}

func (s *spaceStorage) SpaceSettingsId() string {
	return s.spaceSettingsId
}

func (s *spaceStorage) TreeStorage(id string) (storage.TreeStorage, error) {
	return newTreeStorage(s.objDb, s.spaceId, id)
}

func (s *spaceStorage) CreateTreeStorage(payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	return createTreeStorage(s.objDb, s.spaceId, payload)
}

func (s *spaceStorage) AclStorage() (liststorage.ListStorage, error) {
	return s.aclStorage, nil
}

func (s *spaceStorage) SpaceHeader() (header *spacesyncproto.RawSpaceHeaderWithId, err error) {
	return s.header, nil
}

func (s *spaceStorage) StoredIds() (ids []string, err error) {
	err = s.objDb.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = s.keys.TreeRootPrefix()

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			id := make([]byte, 0, len(item.Key()))
			id = item.KeyCopy(id)
			if len(id) <= len(s.keys.TreeRootPrefix())+1 {
				continue
			}
			id = id[len(s.keys.TreeRootPrefix())+1:]
			ids = append(ids, string(id))
		}
		return nil
	})
	return
}

func (s *spaceStorage) SetTreeDeletedStatus(id, status string) (err error) {
	return s.objDb.Update(func(txn *badger.Txn) error {
		return txn.Set(s.keys.TreeDeletedKey(id), []byte(status))
	})
}

func (s *spaceStorage) TreeDeletedStatus(id string) (status string, err error) {
	err = s.objDb.View(func(txn *badger.Txn) error {
		res, err := getTxn(txn, s.keys.TreeDeletedKey(id))
		if err != nil {
			return err
		}
		status = string(res)
		return nil
	})
	if err == badger.ErrKeyNotFound {
		err = nil
	}
	return
}

func (s *spaceStorage) Close() (err error) {
	return nil
}
