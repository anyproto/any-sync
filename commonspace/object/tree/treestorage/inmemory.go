package treestorage

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/util/slice"
	"sync"
)

type InMemoryTreeStorage struct {
	id      string
	root    *treechangeproto.RawTreeChangeWithId
	heads   []string
	changes map[string]*treechangeproto.RawTreeChangeWithId

	sync.RWMutex
}

func (t *InMemoryTreeStorage) TransactionAdd(changes []*treechangeproto.RawTreeChangeWithId, heads []string) error {
	t.RLock()
	defer t.RUnlock()

	for _, ch := range changes {
		t.changes[ch.Id] = ch
	}
	t.heads = append(t.heads[:0], heads...)
	return nil
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

	return &InMemoryTreeStorage{
		id:      root.Id,
		root:    root,
		heads:   append([]string(nil), heads...),
		changes: allChanges,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (t *InMemoryTreeStorage) HasChange(ctx context.Context, id string) (bool, error) {
	_, exists := t.changes[id]
	return exists, nil
}

func (t *InMemoryTreeStorage) Id() string {
	t.RLock()
	defer t.RUnlock()
	return t.id
}

func (t *InMemoryTreeStorage) Root() (*treechangeproto.RawTreeChangeWithId, error) {
	t.RLock()
	defer t.RUnlock()
	return t.root, nil
}

func (t *InMemoryTreeStorage) Heads() ([]string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.heads, nil
}

func (t *InMemoryTreeStorage) SetHeads(heads []string) error {
	t.Lock()
	defer t.Unlock()
	t.heads = append(t.heads[:0], heads...)
	return nil
}

func (t *InMemoryTreeStorage) AddRawChange(change *treechangeproto.RawTreeChangeWithId) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.changes[change.Id] = change
	return nil
}

func (t *InMemoryTreeStorage) GetRawChange(ctx context.Context, changeId string) (*treechangeproto.RawTreeChangeWithId, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}

func (t *InMemoryTreeStorage) Delete() error {
	return nil
}

func (t *InMemoryTreeStorage) Copy() *InMemoryTreeStorage {
	var changes []*treechangeproto.RawTreeChangeWithId
	for _, ch := range t.changes {
		changes = append(changes, ch)
	}
	other, _ := NewInMemoryTreeStorage(t.root, t.heads, changes)
	return other.(*InMemoryTreeStorage)
}

func (t *InMemoryTreeStorage) Equal(other *InMemoryTreeStorage) bool {
	if !slice.UnsortedEquals(t.heads, other.heads) {
		return false
	}
	if len(t.changes) != len(other.changes) {
		return false
	}
	for k, v := range t.changes {
		if otherV, exists := other.changes[k]; exists {
			if otherV.Id == v.Id {
				continue
			}
		}
		return false
	}
	return true
}
