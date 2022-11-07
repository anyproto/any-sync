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

var (
	ErrIncorrectCID = errors.New("incorrect CID")
	ErrInconsistent = errors.New("inconsistent record")
)

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

type ACLList interface {
	RWLocker
	ID() string
	Root() *aclrecordproto.ACLRoot
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
	root    *aclrecordproto.ACLRoot
	records []*ACLRecord
	indexes map[string]int
	id      string

	recBuilder ACLRecordBuilder
	builder    *aclStateBuilder
	aclState   *ACLState
	keychain   *common.Keychain

	sync.RWMutex
}

func BuildACLListWithIdentity(acc *account.AccountData, storage storage.ListStorage) (ACLList, error) {
	builder := newACLStateBuilderWithIdentity(acc)
	return build(storage.Id(), builder, newACLRecordBuilder(storage.Id(), common.NewKeychain()), storage)
}

func BuildACLList(storage storage.ListStorage) (ACLList, error) {
	return build(storage.Id(), newACLStateBuilder(), newACLRecordBuilder(storage.Id(), common.NewKeychain()), storage)
}

func build(id string, stateBuilder *aclStateBuilder, recBuilder ACLRecordBuilder, storage storage.ListStorage) (list ACLList, err error) {
	// TODO: need to add context here
	rootWithId, err := storage.Root()
	if err != nil {
		return
	}
	aclRecRoot, err := recBuilder.ConvertFromRaw(rootWithId)
	if err != nil {
		return
	}

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

	for record.PrevId != "" && record.PrevId != id {
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
	// adding root in the end, because we already parsed it
	records = append(records, aclRecRoot)

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

	list = &aclList{
		root:       aclRecRoot.Model.(*aclrecordproto.ACLRoot),
		records:    records,
		indexes:    indexes,
		builder:    stateBuilder,
		recBuilder: recBuilder,
		aclState:   state,
		id:         id,
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
	record, err := a.recBuilder.ConvertFromRaw(rawRec)
	if err != nil {
		return
	}
	var lastId string
	if len(a.records) > 0 {
		lastId = a.records[len(a.records)-1].Id
	}
	if record.PrevId != lastId {
		return false, ErrInconsistent
	}
	a.records = append(a.records, record)
	a.indexes[record.Id] = len(a.records) - 1
	return true, nil
}

func (a *aclList) IsValidNext(rawRec *aclrecordproto.RawACLRecordWithId) (err error) {
	rec, err := a.recBuilder.ConvertFromRaw(rawRec)
	if err != nil {
		return
	}
	var lastId string
	if len(a.records) > 0 {
		lastId = a.records[len(a.records)-1].Id
	}
	if rec.PrevId != lastId {
		return ErrInconsistent
	}
	return
}

func (a *aclList) ID() string {
	return a.id
}

func (a *aclList) Root() *aclrecordproto.ACLRoot {
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
