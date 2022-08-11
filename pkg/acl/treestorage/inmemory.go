package treestorage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"sync"
)

type inMemoryTreeStorage struct {
	id      string
	header  *treepb.TreeHeader
	heads   []string
	orphans []string
	changes map[string]*aclpb.RawChange

	sync.RWMutex
}

type CreatorFunc = func(string, *treepb.TreeHeader, []*aclpb.RawChange) (TreeStorage, error)

func NewInMemoryTreeStorage(
	treeId string,
	header *treepb.TreeHeader,
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

func (t *inMemoryTreeStorage) TreeID() (string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.id, nil
}

func (t *inMemoryTreeStorage) Header() (*treepb.TreeHeader, error) {
	t.RLock()
	defer t.RUnlock()
	return t.header, nil
}

func (t *inMemoryTreeStorage) Heads() ([]string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.heads, nil
}

func (t *inMemoryTreeStorage) Orphans() ([]string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.orphans, nil
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

func (t *inMemoryTreeStorage) RemoveOrphans(orphans ...string) error {
	t.Lock()
	defer t.Unlock()
	t.orphans = slice.Difference(t.orphans, orphans)
	return nil
}

func (t *inMemoryTreeStorage) AddOrphans(orphans ...string) error {
	t.Lock()
	defer t.Unlock()
	t.orphans = append(t.orphans, orphans...)
	return nil
}

func (t *inMemoryTreeStorage) AddRawChange(change *aclpb.RawChange) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.changes[change.Id] = change
	return nil
}

func (t *inMemoryTreeStorage) AddChange(change aclchanges.Change) error {
	t.Lock()
	defer t.Unlock()
	signature := change.Signature()
	id := change.CID()

	fullMarshalledChange, err := change.ProtoChange().Marshal()
	if err != nil {
		return err
	}
	rawChange := &aclpb.RawChange{
		Payload:   fullMarshalledChange,
		Signature: signature,
		Id:        id,
	}
	t.changes[id] = rawChange
	return nil
}

func (t *inMemoryTreeStorage) GetChange(ctx context.Context, changeId string) (*aclpb.RawChange, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}

type inMemoryTreeStorageProvider struct {
	trees map[string]TreeStorage
	sync.RWMutex
}

func (i *inMemoryTreeStorageProvider) TreeStorage(treeId string) (TreeStorage, error) {
	i.RLock()
	defer i.RUnlock()
	if tree, exists := i.trees[treeId]; exists {
		return tree, nil
	}
	return nil, ErrUnknownTreeId
}

func (i *inMemoryTreeStorageProvider) CreateTreeStorage(treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange) (TreeStorage, error) {
	i.Lock()
	defer i.Unlock()
	res, err := NewInMemoryTreeStorage(treeId, header, changes)
	if err != nil {
		return nil, err
	}

	i.trees[treeId] = res
	return res, nil
}

func NewInMemoryTreeStorageProvider() Provider {
	return &inMemoryTreeStorageProvider{
		trees: make(map[string]TreeStorage),
	}
}
