package storage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"sync"
)

type inMemoryACLListStorage struct {
	id      string
	root    *aclrecordproto.RawACLRecordWithId
	head    string
	records map[string]*aclrecordproto.RawACLRecordWithId

	sync.RWMutex
}

func NewInMemoryACLListStorage(
	id string,
	records []*aclrecordproto.RawACLRecordWithId) (ListStorage, error) {

	allRecords := make(map[string]*aclrecordproto.RawACLRecordWithId)
	for _, ch := range records {
		allRecords[ch.Id] = ch
	}
	root := records[0]
	head := records[len(records)-1]

	return &inMemoryACLListStorage{
		id:      root.Id,
		root:    root,
		head:    head.Id,
		records: allRecords,
	}, nil
}

func (t *inMemoryACLListStorage) Id() string {
	t.RLock()
	defer t.RUnlock()
	return t.id
}

func (t *inMemoryACLListStorage) Root() (*aclrecordproto.RawACLRecordWithId, error) {
	t.RLock()
	defer t.RUnlock()
	return t.root, nil
}

func (t *inMemoryACLListStorage) Head() (string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.head, nil
}

func (t *inMemoryACLListStorage) SetHead(head string) error {
	t.Lock()
	defer t.Unlock()
	t.head = head
	return nil
}

func (t *inMemoryACLListStorage) AddRawRecord(ctx context.Context, record *aclrecordproto.RawACLRecordWithId) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.records[record.Id] = record
	return nil
}

func (t *inMemoryACLListStorage) GetRawRecord(ctx context.Context, recordId string) (*aclrecordproto.RawACLRecordWithId, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.records[recordId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get record with id: %s", recordId)
}

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

type inMemoryStorageProvider struct {
	objects map[string]TreeStorage
	sync.RWMutex
}

func (i *inMemoryStorageProvider) TreeStorage(id string) (TreeStorage, error) {
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
	res, err := NewInMemoryTreeStorage(payload.RootRawChange, payload.Heads, payload.Changes)
	if err != nil {
		return nil, err
	}

	i.objects[payload.RootRawChange.Id] = res
	return res, nil
}

func NewInMemoryTreeStorageProvider() Provider {
	return &inMemoryStorageProvider{
		objects: make(map[string]TreeStorage),
	}
}
