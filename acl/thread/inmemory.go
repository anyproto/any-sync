package thread

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/acl/aclchanges"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/acl/thread/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
	"sync"
)

type inMemoryThread struct {
	id      string
	header  *pb.ThreadHeader
	heads   []string
	orphans []string
	changes map[string]*RawChange

	sync.RWMutex
}

func NewInMemoryThread(firstChange *RawChange) (Thread, error) {
	header := &pb.ThreadHeader{
		FirstChangeId: firstChange.Id,
		IsWorkspace:   false,
	}
	marshalledHeader, err := proto.Marshal(header)
	if err != nil {
		return nil, err
	}
	threadId, err := cid.NewCIDFromBytes(marshalledHeader)
	if err != nil {
		return nil, err
	}

	changes := make(map[string]*RawChange)
	changes[firstChange.Id] = firstChange

	return &inMemoryThread{
		id:      threadId,
		header:  header,
		heads:   []string{firstChange.Id},
		orphans: nil,
		changes: changes,
		RWMutex: sync.RWMutex{},
	}, nil
}

func (t *inMemoryThread) ID() string {
	t.RLock()
	defer t.RUnlock()
	return t.id
}

func (t *inMemoryThread) Header() *pb.ThreadHeader {
	t.RLock()
	defer t.RUnlock()
	return t.header
}

func (t *inMemoryThread) Heads() []string {
	t.RLock()
	defer t.RUnlock()
	return t.heads
}

func (t *inMemoryThread) Orphans() []string {
	t.RLock()
	defer t.RUnlock()
	return t.orphans
}

func (t *inMemoryThread) SetHeads(heads []string) {
	t.Lock()
	defer t.Unlock()
	t.heads = t.heads[:0]

	for _, h := range heads {
		t.heads = append(t.heads, h)
	}
}

func (t *inMemoryThread) RemoveOrphans(orphans ...string) {
	t.Lock()
	defer t.Unlock()
	t.orphans = slice.Difference(t.orphans, orphans)
}

func (t *inMemoryThread) AddOrphans(orphans ...string) {
	t.Lock()
	defer t.Unlock()
	t.orphans = append(t.orphans, orphans...)
}

func (t *inMemoryThread) AddRawChange(change *RawChange) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.changes[change.Id] = change
	return nil
}

func (t *inMemoryThread) AddChange(change aclchanges.Change) error {
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

func (t *inMemoryThread) GetChange(ctx context.Context, changeId string) (*RawChange, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.changes[changeId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get change with id: %s", changeId)
}
