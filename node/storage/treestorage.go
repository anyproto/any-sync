package storage

import (
	"context"
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
)

type treeStorage struct {
	db   *pogreb.DB
	keys treeKeys
	id   string
	root *treechangeproto.RawTreeChangeWithId
}

func newTreeStorage(db *pogreb.DB, treeId string) (ts storage.TreeStorage, err error) {
	keys := newTreeKeys(treeId)
	heads, err := db.Get(keys.HeadsKey())
	if err != nil {
		return
	}
	if heads == nil {
		err = storage.ErrUnknownTreeId
		return
	}

	root, err := db.Get(keys.RawChangeKey(treeId))
	if err != nil {
		return
	}
	if root == nil {
		err = storage.ErrUnknownTreeId
		return
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
	return
}

func createTreeStorage(db *pogreb.DB, payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	keys := newTreeKeys(payload.TreeId)
	has, err := db.Has(keys.HeadsKey())
	if err != nil {
		return
	}
	if has {
		err = storage.ErrTreeExists
		return
	}

	heads := storage.CreateHeadsPayload(payload.Heads)

	for _, ch := range payload.Changes {
		err = db.Put(keys.RawChangeKey(ch.Id), ch.GetRawChange())
		if err != nil {
			return
		}
	}

	err = db.Put(keys.RawChangeKey(payload.RootRawChange.Id), payload.RootRawChange.GetRawChange())
	if err != nil {
		return
	}

	err = db.Put(keys.HeadsKey(), heads)
	if err != nil {
		return
	}

	ts = &treeStorage{
		db:   db,
		keys: keys,
		id:   payload.RootRawChange.Id,
		root: payload.RootRawChange,
	}
	return
}

func (t *treeStorage) Id() string {
	return t.id
}

func (t *treeStorage) Root() (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return t.root, nil
}

func (t *treeStorage) Heads() (heads []string, err error) {
	headsBytes, err := t.db.Get(t.keys.HeadsKey())
	if err != nil {
		return
	}
	if headsBytes == nil {
		err = storage.ErrUnknownTreeId
		return
	}
	heads = storage.ParseHeads(headsBytes)
	return
}

func (t *treeStorage) SetHeads(heads []string) (err error) {
	payload := storage.CreateHeadsPayload(heads)
	return t.db.Put(t.keys.HeadsKey(), payload)
}

func (t *treeStorage) AddRawChange(change *treechangeproto.RawTreeChangeWithId) (err error) {
	return t.db.Put(t.keys.RawChangeKey(change.Id), change.RawChange)
}

func (t *treeStorage) GetRawChange(ctx context.Context, id string) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	res, err := t.db.Get(t.keys.RawChangeKey(id))
	if err != nil {
		return
	}
	if res == nil {
		err = storage.ErrUnknownChange
	}

	raw = &treechangeproto.RawTreeChangeWithId{
		RawChange: res,
		Id:        id,
	}
	return
}

func (t *treeStorage) HasChange(ctx context.Context, id string) (bool, error) {
	return t.db.Has(t.keys.RawChangeKey(id))
}

func (t *treeStorage) Delete() (err error) {
	storedKeys, err := t.storedKeys()
	if err != nil {
		return
	}
	for _, k := range storedKeys {
		err = t.db.Delete(k)
		if err != nil {
			return
		}
	}
	return
}

func (t *treeStorage) storedKeys() (keys [][]byte, err error) {
	index := t.db.Items()

	key, _, err := index.Next()
	for err == nil {
		strKey := string(key)
		if t.keys.isTreeRecordKey(strKey) {
			keys = append(keys, key)
		}
		key, _, err = index.Next()
	}

	if err != pogreb.ErrIterationDone {
		return
	}
	err = nil
	return
}
