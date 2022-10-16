package storage

import (
	"context"
	provider "github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
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

		root, err := provider.GetAndCopy(txn, keys.RawChangeKey(treeId))
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
	return
}

func createTreeStorage(db *badger.DB, spaceId string, payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	keys := newTreeKeys(spaceId, payload.TreeId)
	if provider.Has(db, keys.RootIdKey()) {
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

func (t *treeStorage) ID() (string, error) {
	return t.id, nil
}

func (t *treeStorage) Root() (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return t.root, nil
}

func (t *treeStorage) Heads() (heads []string, err error) {
	headsBytes, err := provider.Get(t.db, t.keys.HeadsKey())
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
	return provider.Put(t.db, t.keys.HeadsKey(), payload)
}

func (t *treeStorage) AddRawChange(change *treechangeproto.RawTreeChangeWithId) (err error) {
	return provider.Put(t.db, t.keys.RawChangeKey(change.Id), change.RawChange)
}

func (t *treeStorage) GetRawChange(ctx context.Context, id string) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	res, err := provider.Get(t.db, t.keys.RawChangeKey(id))
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
	return provider.Has(t.db, t.keys.RawChangeKey(id)), nil
}
