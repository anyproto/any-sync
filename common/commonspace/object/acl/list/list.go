//go:generate mockgen -destination mock_list/mock_list.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list ACLList
package list

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/accountdata"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/liststorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/keychain"
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
	Head() *ACLRecord
	Get(id string) (*ACLRecord, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)

	AddRawRecord(rawRec *aclrecordproto.RawACLRecordWithId) (added bool, err error)

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
	keychain      *keychain.Keychain
	storage       liststorage.ListStorage

	sync.RWMutex
}

func BuildACLListWithIdentity(acc *accountdata.AccountData, storage liststorage.ListStorage) (ACLList, error) {
	builder := newACLStateBuilderWithIdentity(acc)
	return build(storage.Id(), builder, newACLRecordBuilder(storage.Id(), keychain.NewKeychain()), storage)
}

func BuildACLList(storage liststorage.ListStorage) (ACLList, error) {
	return build(storage.Id(), newACLStateBuilder(), newACLRecordBuilder(storage.Id(), keychain.NewKeychain()), storage)
}

func build(id string, stateBuilder *aclStateBuilder, recBuilder ACLRecordBuilder, storage liststorage.ListStorage) (list ACLList, err error) {
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

	// TODO: check if this is correct (raw model instead of unmarshalled)
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
	}
	return
}

func (a *aclList) Records() []*ACLRecord {
	return a.records
}

func (a *aclList) AddRawRecord(rawRec *aclrecordproto.RawACLRecordWithId) (added bool, err error) {
	if _, ok := a.indexes[rawRec.Id]; ok {
		return
	}
	record, err := a.recordBuilder.ConvertFromRaw(rawRec)
	if err != nil {
		return
	}
	if err = a.aclState.applyRecord(record); err != nil {
		return
	}
	a.records = append(a.records, record)
	a.indexes[record.Id] = len(a.records) - 1
	if err = a.storage.AddRawRecord(context.Background(), rawRec); err != nil {
		return
	}
	if err = a.storage.SetHead(rawRec.Id); err != nil {
		return
	}
	return true, nil
}

func (a *aclList) IsValidNext(rawRec *aclrecordproto.RawACLRecordWithId) (err error) {
	_, err = a.recordBuilder.ConvertFromRaw(rawRec)
	if err != nil {
		return
	}
	// TODO: change state and add "check" method for records
	return
}

func (a *aclList) ID() string {
	return a.id
}

func (a *aclList) Root() *aclrecordproto.RawACLRecordWithId {
	return a.root
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
