package storage

import (
	"bytes"
	"context"
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
	"github.com/gogo/protobuf/proto"
	"strings"
	"sync"
)

type treeStorage struct {
	db       *pogreb.DB
	keys     treeKeys
	id       string
	rootKey  []byte
	headsKey []byte
	heads    []string
	root     *treechangeproto.RawTreeChangeWithId
	headsMx  sync.Mutex
}

func newTreeStorage(db *pogreb.DB, treeId string) (ts storage.TreeStorage, err error) {
	keys := treeKeys{treeId}
	heads, err := db.Get(keys.HeadsKey())
	if err != nil {
		return
	}
	if heads == nil {
		err = storage.ErrUnknownTreeId
		return
	}

	res, err := db.Get(keys.RootKey())
	if err != nil {
		return
	}
	if res == nil {
		err = storage.ErrUnknownTreeId
		return
	}

	root := &treechangeproto.RawTreeChangeWithId{}
	err = proto.Unmarshal(res, root)
	if err != nil {
		return
	}

	ts = &treeStorage{
		db:       db,
		keys:     keys,
		rootKey:  keys.RootKey(),
		headsKey: keys.HeadsKey(),
		id:       treeId,
		heads:    parseHeads(heads),
		root:     root,
	}
	return
}

func createTreeStorage(db *pogreb.DB, payload storage.TreeStorageCreatePayload) (ts storage.TreeStorage, err error) {
	keys := treeKeys{id: payload.TreeId}
	has, err := db.Has(keys.RootKey())
	if err != nil {
		return
	}
	if !has {
		err = storage.ErrUnknownTreeId
		return
	}

	heads := createHeadsPayload(payload.Heads)

	for _, ch := range payload.Changes {
		err = db.Put(keys.RawChangeKey(ch.Id), ch.GetRawChange())
		if err != nil {
			return
		}
	}

	err = db.Put(keys.HeadsKey(), heads)
	if err != nil {
		return
	}

	// duplicating same change in raw changes
	err = db.Put(keys.RawChangeKey(payload.TreeId), payload.RootRawChange.GetRawChange())
	if err != nil {
		return
	}

	err = db.Put(keys.RootKey(), payload.RootRawChange.GetRawChange())
	if err != nil {
		return
	}

	ts = &treeStorage{
		db:       db,
		keys:     keys,
		rootKey:  keys.RootKey(),
		headsKey: keys.HeadsKey(),
		id:       payload.TreeId,
		heads:    payload.Heads,
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

func (t *treeStorage) Heads() ([]string, error) {
	t.headsMx.Lock()
	defer t.headsMx.Unlock()
	return t.heads, nil
}

func (t *treeStorage) SetHeads(heads []string) (err error) {
	t.headsMx.Lock()
	defer t.headsMx.Unlock()
	defer func() {
		if err == nil {
			t.heads = heads
		}
	}()
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
