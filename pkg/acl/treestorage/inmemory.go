package treestorage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
	"sync"
)

type inMemoryTreeStorage struct {
	id      string
	header  *pb.TreeHeader
	heads   []string
	orphans []string
	changes map[string]*RawChange

	sync.RWMutex
}

func NewInMemoryTreeStorage(firstChange *RawChange) (TreeStorage, error) {
	header := &pb.TreeHeader{
		FirstChangeId: firstChange.Id,
		IsWorkspace:   false,
	}
	marshalledHeader, err := proto.Marshal(header)
	if err != nil {
		return nil, err
	}
	treeId, err := cid.NewCIDFromBytes(marshalledHeader)
	if err != nil {
		return nil, err
	}

	changes := make(map[string]*RawChange)
	changes[firstChange.Id] = firstChange

	return &inMemoryTreeStorage{
		id:      treeId,
		header:  header,
		heads:   []string{firstChange.Id},
		orphans: nil,
		changes: changes,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (t *inMemoryTreeStorage) TreeID() (string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.id, nil
}

func (t *inMemoryTreeStorage) Header() (*pb.TreeHeader, error) {
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

func (t *inMemoryTreeStorage) AddRawChange(change *RawChange) error {
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
	aclChange := change.ProtoChange()

	fullMarshalledChange, err := proto.Marshal(aclChange)
	if err != nil {
		return err
	}
	rawChange := &RawChange{
		Payload:   fullMarshalledChange,
		Signature: signature,
		Id:        id,
	}
	t.changes[id] = rawChange
	return nil
}

func (t *inMemoryTreeStorage) GetChange(ctx context.Context, changeId string) (*RawChange, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}
