package treestorage

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"sync"
)

type inMemoryTreeStorage struct {
	id      string
	root    *treechangeproto.RawTreeChangeWithId
	heads   []string
	changes map[string]*treechangeproto.RawTreeChangeWithId

	sync.RWMutex
}

func NewInMemoryTreeStorage(
	root *treechangeproto.RawTreeChangeWithId,
	heads []string,
	changes []*treechangeproto.RawTreeChangeWithId) (TreeStorage, error) {
	allChanges := make(map[string]*treechangeproto.RawTreeChangeWithId)
	for _, ch := range changes {
		allChanges[ch.Id] = ch
	}
	allChanges[root.Id] = root

	return &inMemoryTreeStorage{
		id:      root.Id,
		root:    root,
		heads:   heads,
		changes: allChanges,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (t *inMemoryTreeStorage) HasChange(ctx context.Context, id string) (bool, error) {
	_, exists := t.changes[id]
	return exists, nil
}

func (t *inMemoryTreeStorage) Id() string {
	t.RLock()
	defer t.RUnlock()
	return t.id
}

func (t *inMemoryTreeStorage) Root() (*treechangeproto.RawTreeChangeWithId, error) {
	t.RLock()
	defer t.RUnlock()
	return t.root, nil
}

func (t *inMemoryTreeStorage) Heads() ([]string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.heads, nil
}

func (t *inMemoryTreeStorage) SetHeads(heads []string) error {
	t.Lock()
	defer t.Unlock()
	t.heads = t.heads[:0]

	for _, h := range heads {
		t.heads = append(t.heads, h)
	}
	return nil
}

func (t *inMemoryTreeStorage) AddRawChange(change *treechangeproto.RawTreeChangeWithId) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.changes[change.Id] = change
	return nil
}

func (t *inMemoryTreeStorage) GetRawChange(ctx context.Context, changeId string) (*treechangeproto.RawTreeChangeWithId, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}

func (t *inMemoryTreeStorage) Delete() error {
	return nil
}
