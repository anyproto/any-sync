package storage

import (
	"bytes"
	"context"
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
	"strings"
)

type treeStorage struct {
	db       *pogreb.DB
	keys     treeKeys
	id       string
	headsKey []byte
	root     *treechangeproto.RawTreeChangeWithId
}

func newTreeStorage(db *pogreb.DB, treeId string) (ts storage.TreeStorage, err error) {
	keys := treeKeys{treeId}
	has, err := db.Has(keys.RootIdKey())
	if err != nil {
		return
	}
	if !has {
		err = storage.ErrUnknownTreeId
		return
	}

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
	if err != nil {
		return
	}

	ts = &treeStorage{
		db:       db,
		keys:     keys,
		headsKey: keys.HeadsKey(),
		id:       treeId,
		root:     rootWithId,
	}
	return
}

func createTreeStorage(db *pogreb.DB, payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	keys := treeKeys{id: payload.TreeId}
	has, err := db.Has(keys.RootIdKey())
	if err != nil {
		return
	}
	if has {
		err = storage.ErrTreeExists
		return
	}

	heads := createHeadsPayload(payload.Heads)

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

	err = db.Put(keys.RootIdKey(), []byte(payload.RootRawChange.Id))
	if err != nil {
		return
	}

	ts = &treeStorage{
		db:       db,
		keys:     keys,
		headsKey: keys.HeadsKey(),
		id:       payload.RootRawChange.Id,
		root:     payload.RootRawChange,
	}
	return
}

func (t *treeStorage) ID() (string, error) {
	return t.id, nil
}

func (t *treeStorage) Root() (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return t.root, nil
}

func (t *treeStorage) Heads() (heads []string, err error) {
	headsBytes, err := t.db.Get(t.keys.HeadsKey())
	if err != nil {
		return
	}
	if heads == nil {
		err = storage.ErrUnknownTreeId
		return
	}
	heads = parseHeads(headsBytes)
	return
}

func (t *treeStorage) SetHeads(heads []string) (err error) {
	payload := createHeadsPayload(heads)
	return t.db.Put(t.headsKey, payload)
}

func (t *treeStorage) AddRawChange(change *treechangeproto.RawTreeChangeWithId) (err error) {
	return t.db.Put([]byte(change.Id), change.RawChange)
}

func (t *treeStorage) GetRawChange(ctx context.Context, id string) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	res, err := t.db.Get(t.keys.RawChangeKey(id))
	if err != nil {
		return
	}
	if res == nil {
		err = storage.ErrUnkownChange
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

func parseHeads(headsPayload []byte) []string {
	return strings.Split(string(headsPayload), "/")
}

func createHeadsPayload(heads []string) []byte {
	var (
		b        bytes.Buffer
		totalLen int
	)
	for _, s := range heads {
		totalLen += len(s)
	}
	// adding separators
	totalLen += len(heads) - 1
	b.Grow(totalLen)
	for idx, s := range heads {
		if idx > 0 {
			b.WriteString("/")
		}
		b.WriteString(s)
	}
	return b.Bytes()
}
