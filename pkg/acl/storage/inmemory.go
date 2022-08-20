package storage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"sync"
)

type inMemoryTreeStorage struct {
	id      string
	header  *aclpb.Header
	heads   []string
	orphans []string
	changes map[string]*aclpb.RawChange

	sync.RWMutex
}

type CreatorFunc = func(string, *aclpb.Header, []*aclpb.RawChange) (TreeStorage, error)

func NewInMemoryTreeStorage(
	treeId string,
	header *aclpb.Header,
	changes []*aclpb.RawChange) (TreeStorage, error) {
	allChanges := make(map[string]*aclpb.RawChange)
	var orphans []string
	for _, ch := range changes {
		allChanges[ch.Id] = ch
		orphans = append(orphans, ch.Id)
	}

	return &inMemoryTreeStorage{
		id:      treeId,
		header:  header,
		heads:   nil,
		orphans: orphans,
		changes: allChanges,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (t *inMemoryTreeStorage) ID() (string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.id, nil
}

func (t *inMemoryTreeStorage) Header() (*aclpb.Header, error) {
	t.RLock()
	defer t.RUnlock()
	return t.header, nil
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

func (t *inMemoryTreeStorage) AddRawChange(change *aclpb.RawChange) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.changes[change.Id] = change
	return nil
}

func (t *inMemoryTreeStorage) GetRawChange(ctx context.Context, changeId string) (*aclpb.RawChange, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}

type inMemoryStorageProvider struct {
	objects map[string]Storage
	sync.RWMutex
}

func (i *inMemoryStorageProvider) Storage(id string) (Storage, error) {
	i.RLock()
	defer i.RUnlock()
	if tree, exists := i.objects[id]; exists {
		return tree, nil
	}
	return nil, ErrUnknownTreeId
}

func (i *inMemoryStorageProvider) CreateTreeStorage(treeId string, header *aclpb.Header, changes []*aclpb.RawChange) (TreeStorage, error) {
	i.Lock()
	defer i.Unlock()
	res, err := NewInMemoryTreeStorage(treeId, header, changes)
	if err != nil {
		return nil, err
	}

	i.objects[treeId] = res
	return res, nil
}

func NewInMemoryTreeStorageProvider() Provider {
	return &inMemoryStorageProvider{
		objects: make(map[string]Storage),
	}
}
