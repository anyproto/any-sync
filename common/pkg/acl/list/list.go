//go:generate mockgen -destination mock_list/mock_list.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list ACLList
package list

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"sync"
)

type IterFunc = func(record *ACLRecord) (IsContinue bool)

var ErrIncorrectCID = errors.New("incorrect CID")

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

type ACLList interface {
	RWLocker
	ID() string
	Root() *aclrecordproto.RawACLRecordWithId
	Records() []*ACLRecord
	ACLState() *ACLState
	IsAfter(first string, second string) (bool, error)
	AddRawRecords(ctx context.Context, rec []*aclrecordproto.RawACLRecordWithId) (err error)
	Head() *ACLRecord
	Get(id string) (*ACLRecord, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)
	Close() (err error)
}

type aclList struct {
	root    *aclrecordproto.RawACLRecordWithId
	records []*ACLRecord
	indexes map[string]int
	id      string

	stateBuilder  *aclStateBuilder
	recordBuilder ACLRecordBuilder
	aclState      *ACLState
	keychain      *common.Keychain
	storage       storage.ListStorage

	sync.RWMutex
}

func BuildACLListWithIdentity(acc *account.AccountData, storage storage.ListStorage) (ACLList, error) {
	id := storage.ID()
	builder := newACLStateBuilderWithIdentity(acc)
	return build(id, builder, newACLRecordBuilder(id, common.NewKeychain()), storage)
}

func BuildACLList(storage storage.ListStorage) (ACLList, error) {
	id := storage.ID()
	return build(id, newACLStateBuilder(), newACLRecordBuilder(id, common.NewKeychain()), storage)
}

func build(id string, stateBuilder *aclStateBuilder, recBuilder ACLRecordBuilder, storage storage.ListStorage) (list ACLList, err error) {
	head, err := storage.Head()
	if err != nil {
		return
	}

	rawRecordWithId, err := storage.GetRawRecord(context.Background(), head)
	if err != nil {
		return
	}

	record, err := recBuilder.ConvertFromRaw(rawRecordWithId)
	if err != nil {
		return
	}
	records := []*ACLRecord{record}

	for record.PrevId != "" {
		rawRecordWithId, err = storage.GetRawRecord(context.Background(), record.PrevId)
		if err != nil {
			return
		}

		record, err = recBuilder.ConvertFromRaw(rawRecordWithId)
		if err != nil {
			return
		}
		records = append(records, record)
	}

	indexes := make(map[string]int)
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
		indexes[records[i].Id] = i
		indexes[records[j].Id] = j
	}
	// adding missed index if needed
	if len(records)%2 != 0 {
		indexes[records[len(records)/2].Id] = len(records) / 2
	}

	stateBuilder.Init(id)
	state, err := stateBuilder.Build(records)
	if err != nil {
		return
	}

	rootWithId, err := storage.Root()
	if err != nil {
		return
	}

	list = &aclList{
		root:          rootWithId,
		records:       records,
		indexes:       indexes,
		stateBuilder:  stateBuilder,
		recordBuilder: recBuilder,
		aclState:      state,
		storage:       storage,
		id:            id,
		RWMutex:       sync.RWMutex{},
	}
	return
}

func (a *aclList) Records() []*ACLRecord {
	return a.records
}

func (a *aclList) ID() string {
	return a.id
}

func (a *aclList) Root() *aclrecordproto.RawACLRecordWithId {
	return a.root
}

func (a *aclList) AddRawRecords(ctx context.Context, records []*aclrecordproto.RawACLRecordWithId) (err error) {
	if len(records) == 0 {
		return
	}
	// converting and verifying
	var aclRecords []*ACLRecord
	for _, rec := range records {
		var record *ACLRecord
		record, err = a.recordBuilder.ConvertFromRaw(rec)
		if err != nil {
			return
		}
		aclRecords = append(aclRecords, record)
	}

	// trying to append them to state
	err = a.stateBuilder.Append(a.aclState, aclRecords)
	if err != nil {
		return
	}

	// saving to storage
	for _, rec := range records {
		err = a.storage.AddRawRecord(ctx, rec)
		if err != nil {
			return
		}
	}

	// setting new head
	err = a.storage.SetHead(records[len(records)-1].Id)
	return
}

func (a *aclList) ACLState() *ACLState {
	return a.aclState
}

func (a *aclList) IsAfter(first string, second string) (bool, error) {
	firstRec, okFirst := a.indexes[first]
	secondRec, okSecond := a.indexes[second]
	if !okFirst || !okSecond {
		return false, fmt.Errorf("not all entries are there: first (%t), second (%t)", okFirst, okSecond)
	}
	return firstRec >= secondRec, nil
}

func (a *aclList) Head() *ACLRecord {
	return a.records[len(a.records)-1]
}

func (a *aclList) Get(id string) (*ACLRecord, error) {
	recIdx, ok := a.indexes[id]
	if !ok {
		return nil, fmt.Errorf("no such record")
	}
	return a.records[recIdx], nil
}

func (a *aclList) Iterate(iterFunc IterFunc) {
	for _, rec := range a.records {
		if !iterFunc(rec) {
			return
		}
	}
}

func (a *aclList) IterateFrom(startId string, iterFunc IterFunc) {
	recIdx, ok := a.indexes[startId]
	if !ok {
		return
	}
	for i := recIdx; i < len(a.records); i++ {
		if !iterFunc(a.records[i]) {
			return
		}
	}
}

func (a *aclList) Close() (err error) {
	return nil
}
