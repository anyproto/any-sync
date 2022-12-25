package liststorage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
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
