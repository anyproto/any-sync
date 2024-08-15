package treestorage

import (
	"context"
	"fmt"
	"sync"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/slice"
)

type InMemoryTreeStorage struct {
	id      string
	root    *treechangeproto.RawTreeChangeWithId
	heads   []string
	Changes map[string]*treechangeproto.RawTreeChangeWithId
	addErr  error

	sync.Mutex
}

func (t *InMemoryTreeStorage) GetAllChangeIds() (chs []string, err error) {
	t.Lock()
	defer t.Unlock()
	chs = make([]string, 0, len(t.Changes))
	for id := range t.Changes {
		chs = append(chs, id)
	}
	return
}

func (t *InMemoryTreeStorage) GetAppendRawChange(ctx context.Context, buf []byte, id string) (*treechangeproto.RawTreeChangeWithId, error) {
	return t.GetRawChange(ctx, id)
}

func (t *InMemoryTreeStorage) SetReturnErrorOnAdd(err error) {
	t.addErr = err
}

func (t *InMemoryTreeStorage) AddRawChangesSetHeads(changes []*treechangeproto.RawTreeChangeWithId, heads []string) error {
	t.Lock()
	defer t.Unlock()
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
	}, nil
}

func (t *InMemoryTreeStorage) HasChange(ctx context.Context, id string) (bool, error) {
	_, exists := t.Changes[id]
	return exists, nil
}

func (t *InMemoryTreeStorage) Id() string {
	t.Lock()
	defer t.Unlock()
	return t.id
}

func (t *InMemoryTreeStorage) Root() (*treechangeproto.RawTreeChangeWithId, error) {
	t.Lock()
	defer t.Unlock()
	return t.root, nil
}

func (t *InMemoryTreeStorage) Heads() ([]string, error) {
	t.Lock()
	defer t.Unlock()
	return t.heads, nil
}

func (t *InMemoryTreeStorage) SetHeads(heads []string) error {
	t.Lock()
	defer t.Unlock()
	t.heads = append(t.heads[:0], heads...)
	return nil
}

func (t *InMemoryTreeStorage) AllChanges() []*treechangeproto.RawTreeChangeWithId {
	var allChanges []*treechangeproto.RawTreeChangeWithId
	for _, ch := range t.Changes {
		allChanges = append(allChanges, ch)
	}
	return allChanges
}

func (t *InMemoryTreeStorage) AddRawChange(change *treechangeproto.RawTreeChangeWithId) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.Changes[change.Id] = change
	return nil
}

func (t *InMemoryTreeStorage) GetRawChange(ctx context.Context, changeId string) (*treechangeproto.RawTreeChangeWithId, error) {
	t.Lock()
	defer t.Unlock()
	if res, exists := t.Changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}

func (t *InMemoryTreeStorage) Delete() error {
	t.Lock()
	defer t.Unlock()
	t.root = nil
	t.Changes = nil
	t.heads = nil
	return nil
}

func (t *InMemoryTreeStorage) Copy() *InMemoryTreeStorage {
	t.Lock()
	defer t.Unlock()
	var changes []*treechangeproto.RawTreeChangeWithId
	for _, ch := range t.Changes {
		changes = append(changes, ch)
	}
	other, _ := NewInMemoryTreeStorage(t.root, t.heads, changes)
	return other.(*InMemoryTreeStorage)
}

func (t *InMemoryTreeStorage) Equal(other *InMemoryTreeStorage) bool {
	t.Lock()
	defer t.Unlock()
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
