package storage

import (
	"context"
	storage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/dgraph-io/badger/v3"
)

type treeStorage struct {
	db   *badger.DB
	keys treeKeys
	id   string
	root *treechangeproto.RawTreeChangeWithId
}

func newTreeStorage(db *badger.DB, spaceId, treeId string) (ts storage.TreeStorage, err error) {
	keys := newTreeKeys(spaceId, treeId)
	err = db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(keys.RootIdKey())
		if err != nil {
			return err
		}

		root, err := getTxn(txn, keys.RawChangeKey(treeId))
		if err != nil {
			return err
		}

		rootWithId := &treechangeproto.RawTreeChangeWithId{
			RawChange: root,
			Id:        treeId,
		}

		ts = &treeStorage{
			db:   db,
			keys: keys,
			id:   treeId,
			root: rootWithId,
		}
		return nil
	})
	if err == badger.ErrKeyNotFound {
		err = storage.ErrUnknownTreeId
	}
	return
}

func createTreeStorage(db *badger.DB, spaceId string, payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	keys := newTreeKeys(spaceId, payload.RootRawChange.Id)
	if hasDB(db, keys.RootIdKey()) {
		err = storage.ErrTreeExists
		return
	}
	err = db.Update(func(txn *badger.Txn) error {
		heads := storage.CreateHeadsPayload(payload.Heads)

		for _, ch := range payload.Changes {
			err = txn.Set(keys.RawChangeKey(ch.Id), ch.GetRawChange())
			if err != nil {
				return err
			}
		}

		err = txn.Set(keys.RawChangeKey(payload.RootRawChange.Id), payload.RootRawChange.GetRawChange())
		if err != nil {
			return err
		}

		err = txn.Set(keys.HeadsKey(), heads)
		if err != nil {
			return err
		}

		err = txn.Set(keys.RootIdKey(), nil)
		if err != nil {
			return err
		}

		ts = &treeStorage{
			db:   db,
			keys: keys,
			id:   payload.RootRawChange.Id,
			root: payload.RootRawChange,
		}
		return nil
	})
	return
}

func (t *treeStorage) ID() string {
	return t.id
}

func (t *treeStorage) Root() (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return t.root, nil
}

func (t *treeStorage) Heads() (heads []string, err error) {
	headsBytes, err := getDB(t.db, t.keys.HeadsKey())
	if err != nil {
		if err == badger.ErrKeyNotFound {
			err = storage.ErrUnknownTreeId
		}
		return
	}
	heads = storage.ParseHeads(headsBytes)
	return
}

func (t *treeStorage) SetHeads(heads []string) (err error) {
	payload := storage.CreateHeadsPayload(heads)
	return putDB(t.db, t.keys.HeadsKey(), payload)
}

func (t *treeStorage) AddRawChange(change *treechangeproto.RawTreeChangeWithId) (err error) {
	return putDB(t.db, t.keys.RawChangeKey(change.Id), change.RawChange)
}

func (t *treeStorage) GetRawChange(ctx context.Context, id string) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	res, err := getDB(t.db, t.keys.RawChangeKey(id))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			err = storage.ErrUnknownTreeId
		}
		return
	}

	raw = &treechangeproto.RawTreeChangeWithId{
		RawChange: res,
		Id:        id,
	}
	return
}

func (t *treeStorage) HasChange(ctx context.Context, id string) (bool, error) {
	return hasDB(t.db, t.keys.RawChangeKey(id)), nil
}
