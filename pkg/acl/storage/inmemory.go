package storage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"sync"
)

type inMemoryACLListStorage struct {
	header  *aclpb.ACLHeader
	records []*aclpb.RawACLRecord

	id string

	sync.RWMutex
}

func NewInMemoryACLListStorage(
	id string,
	header *aclpb.ACLHeader,
	records []*aclpb.RawACLRecord) (ListStorage, error) {
	return &inMemoryACLListStorage{
		id:      id,
		header:  header,
		records: records,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (i *inMemoryACLListStorage) Header() (*aclpb.ACLHeader, error) {
	i.RLock()
	defer i.RUnlock()
	return i.header, nil
}

func (i *inMemoryACLListStorage) Head() (*aclpb.RawACLRecord, error) {
	i.RLock()
	defer i.RUnlock()
	return i.records[len(i.records)-1], nil
}

func (i *inMemoryACLListStorage) GetRawRecord(ctx context.Context, id string) (*aclpb.RawACLRecord, error) {
	i.RLock()
	defer i.RUnlock()
	for _, rec := range i.records {
		if rec.Id == id {
			return rec, nil
		}
	}
	return nil, fmt.Errorf("no such record")
}

func (i *inMemoryACLListStorage) AddRawRecord(ctx context.Context, rec *aclpb.RawACLRecord) error {
	panic("implement me")
}

func (i *inMemoryACLListStorage) ID() (string, error) {
	i.RLock()
	defer i.RUnlock()
	return i.id, nil
}

type inMemoryTreeStorage struct {
	id      string
	header  *aclpb.TreeHeader
	heads   []string
	changes map[string]*aclpb.RawTreeChangeWithId

	sync.RWMutex
}

func NewInMemoryTreeStorage(
	treeId string,
	header *aclpb.TreeHeader,
	heads []string,
	changes []*aclpb.RawTreeChangeWithId) (TreeStorage, error) {
	allChanges := make(map[string]*aclpb.RawTreeChangeWithId)
	for _, ch := range changes {
		allChanges[ch.Id] = ch
	}

	return &inMemoryTreeStorage{
		id:      treeId,
		header:  header,
		heads:   heads,
		changes: allChanges,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (t *inMemoryTreeStorage) ID() (string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.id, nil
}

func (t *inMemoryTreeStorage) Header() (*aclpb.TreeHeader, error) {
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

func (t *inMemoryTreeStorage) AddRawChange(change *aclpb.RawTreeChangeWithId) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.changes[change.Id] = change
	return nil
}

func (t *inMemoryTreeStorage) GetRawChange(ctx context.Context, changeId string) (*aclpb.RawTreeChangeWithId, error) {
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

func (i *inMemoryStorageProvider) AddStorage(id string, st Storage) error {
	i.Lock()
	defer i.Unlock()
	if _, exists := i.objects[id]; exists {
		return fmt.Errorf("storage already exists")
	}

	i.objects[id] = st
	return nil
}

func (i *inMemoryStorageProvider) Storage(id string) (Storage, error) {
	i.RLock()
	defer i.RUnlock()
	if tree, exists := i.objects[id]; exists {
		return tree, nil
	}
	return nil, ErrUnknownTreeId
}

func (i *inMemoryStorageProvider) CreateTreeStorage(payload TreeStorageCreatePayload) (TreeStorage, error) {
	i.Lock()
	defer i.Unlock()
	res, err := NewInMemoryTreeStorage(payload.TreeId, payload.Header, payload.Heads, payload.Changes)
	if err != nil {
		return nil, err
	}

	i.objects[payload.TreeId] = res
	return res, nil
}

func (i *inMemoryStorageProvider) CreateACLListStorage(payload ACLListStorageCreatePayload) (ListStorage, error) {
	i.Lock()
	defer i.Unlock()
	res, err := NewInMemoryACLListStorage(payload.ListId, payload.Header, payload.Records)
	if err != nil {
		return nil, err
	}

	i.objects[payload.ListId] = res
	return res, nil
}

func NewInMemoryTreeStorageProvider() Provider {
	return &inMemoryStorageProvider{
		objects: make(map[string]Storage),
	}
}
