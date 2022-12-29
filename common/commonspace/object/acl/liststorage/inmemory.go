package liststorage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"sync"
)

type inMemoryAclListStorage struct {
	id      string
	root    *aclrecordproto.RawAclRecordWithId
	head    string
	records map[string]*aclrecordproto.RawAclRecordWithId

	sync.RWMutex
}

func NewInMemoryAclListStorage(
	id string,
	records []*aclrecordproto.RawAclRecordWithId) (ListStorage, error) {

	allRecords := make(map[string]*aclrecordproto.RawAclRecordWithId)
	for _, ch := range records {
		allRecords[ch.Id] = ch
	}
	root := records[0]
	head := records[len(records)-1]

	return &inMemoryAclListStorage{
		id:      root.Id,
		root:    root,
		head:    head.Id,
		records: allRecords,
	}, nil
}

func (t *inMemoryAclListStorage) Id() string {
	t.RLock()
	defer t.RUnlock()
	return t.id
}

func (t *inMemoryAclListStorage) Root() (*aclrecordproto.RawAclRecordWithId, error) {
	t.RLock()
	defer t.RUnlock()
	return t.root, nil
}

func (t *inMemoryAclListStorage) Head() (string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.head, nil
}

func (t *inMemoryAclListStorage) SetHead(head string) error {
	t.Lock()
	defer t.Unlock()
	t.head = head
	return nil
}

func (t *inMemoryAclListStorage) AddRawRecord(ctx context.Context, record *aclrecordproto.RawAclRecordWithId) error {
	t.Lock()
	defer t.Unlock()
	// TODO: better to do deep copy
	t.records[record.Id] = record
	return nil
}

func (t *inMemoryAclListStorage) GetRawRecord(ctx context.Context, recordId string) (*aclrecordproto.RawAclRecordWithId, error) {
	t.RLock()
	defer t.RUnlock()
	if res, exists := t.records[recordId]; exists {
		return res, nil
	}
	return nil, fmt.Errorf("could not get record with id: %s", recordId)
}
