package list

import (
	"context"
	"fmt"
	"sync"

	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type inMemoryStorage struct {
	id             string
	root           StorageRecord
	head           string
	recordsToIndex map[string]int
	records        []StorageRecord

	sync.RWMutex
}

func NewInMemoryStorage(
	id string,
	records []*consensusproto.RawRecordWithId) (Storage, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("empty records")
	}
	newRecs := make([]StorageRecord, 0, len(records))
	recordsToIndex := make(map[string]int, len(records))
	newRecs = append(newRecs, StorageRecord{
		RawRecord: records[0].Payload,
		PrevId:    "",
		Id:        records[0].Id,
		Order:     1,
	})
	recordsToIndex[newRecs[0].Id] = 0
	for i := 1; i < len(records)-1; i++ {
		prevRec := newRecs[i-1]
		rec := records[i]
		newRecs = append(newRecs, StorageRecord{
			RawRecord:  rec.Payload,
			PrevId:     prevRec.Id,
			Id:         rec.Id,
			Order:      prevRec.Order + 1,
			ChangeSize: len(rec.Payload),
		})
		recordsToIndex[rec.Id] = i
	}
	root := newRecs[0]
	head := records[len(records)-1]

	return &inMemoryStorage{
		id:             root.Id,
		root:           root,
		head:           head.Id,
		records:        newRecs,
		recordsToIndex: recordsToIndex,
	}, nil
}

func (t *inMemoryStorage) Root(ctx context.Context) (StorageRecord, error) {
	return t.root, nil
}

func (t *inMemoryStorage) Head(ctx context.Context) (string, error) {
	t.RLock()
	defer t.RUnlock()
	return t.head, nil
}

func (t *inMemoryStorage) Has(ctx context.Context, id string) (bool, error) {
	t.RLock()
	defer t.RUnlock()
	_, exists := t.recordsToIndex[id]
	return exists, nil
}

func (t *inMemoryStorage) Get(ctx context.Context, id string) (StorageRecord, error) {
	t.RLock()
	defer t.RUnlock()
	if idx, exists := t.recordsToIndex[id]; exists {
		return t.records[idx], nil
	}
	return StorageRecord{}, ErrNoSuchRecord
}

func (t *inMemoryStorage) GetAfterOrder(ctx context.Context, order int, iter StorageIterator) error {
	t.RLock()
	defer t.RUnlock()
	if order >= len(t.records) {
		return nil
	}
	for i := order; i < len(t.records); i++ {
		if shouldContinue, err := iter(ctx, t.records[i]); !shouldContinue || err != nil {
			return err
		}
	}
	return nil
}

func (t *inMemoryStorage) AddAll(ctx context.Context, records []StorageRecord) error {
	t.Lock()
	defer t.Unlock()

	for _, rec := range records {
		t.records = append(t.records, rec)
		t.recordsToIndex[rec.Id] = len(t.records) - 1
	}
	t.head = records[len(records)-1].Id
	return nil
}

func (t *inMemoryStorage) Id() string {
	return t.id
}
