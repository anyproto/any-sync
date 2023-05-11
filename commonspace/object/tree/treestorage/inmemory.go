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
	Changes map[string]*treechangeproto.RawTreeChangeWithId
	addErr  error

	sync.RWMutex
}

func (t *InMemoryTreeStorage) SetReturnErrorOnAdd(err error) {
	t.addErr = err
}

func (t *InMemoryTreeStorage) AddRawChangesSetHead(changes []*treechangeproto.RawTreeChangeWithId, heads []string) error {
	t.RLock()
	defer t.RUnlock()
	if t.addErr != nil {
		return t.addErr
	}

	for _, ch := range changes {
		t.Changes[ch.Id] = ch
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
		Changes: allChanges,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (t *InMemoryTreeStorage) HasChange(ctx context.Context, id string) (bool, error) {
	_, exists := t.Changes[id]
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
	t.Changes[change.Id] = change
	return nil
}

func (t *InMemoryTreeStorage) GetRawChange(ctx context.Context, changeId string) (*treechangeproto.RawTreeChangeWithId, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.Changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}

func (t *InMemoryTreeStorage) Delete() error {
	return nil
}

func (t *InMemoryTreeStorage) Copy() *InMemoryTreeStorage {
	var changes []*treechangeproto.RawTreeChangeWithId
	for _, ch := range t.Changes {
		changes = append(changes, ch)
	}
	other, _ := NewInMemoryTreeStorage(t.root, t.heads, changes)
	return other.(*InMemoryTreeStorage)
}

func (t *InMemoryTreeStorage) Equal(other *InMemoryTreeStorage) bool {
	if !slice.UnsortedEquals(t.heads, other.heads) {
		return false
	}
	if len(t.Changes) != len(other.Changes) {
		return false
	}
	for k, v := range t.Changes {
		if otherV, exists := other.Changes[k]; exists {
			if otherV.Id == v.Id {
				continue
			}
		}
		return false
	}
	return true
}
