//go:generate mockgen -destination mock_list/mock_list.go github.com/anytypeio/any-sync/commonspace/object/acl/list AclList
package list

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/acl/liststorage"
	"github.com/anytypeio/any-sync/commonspace/object/keychain"
	"github.com/anytypeio/any-sync/util/crypto"
	"sync"
)

type IterFunc = func(record *AclRecord) (IsContinue bool)

var ErrIncorrectCID = errors.New("incorrect CID")

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

type AclList interface {
	RWLocker
	Id() string
	Root() *aclrecordproto.RawAclRecordWithId
	Records() []*AclRecord
	AclState() *AclState
	IsAfter(first string, second string) (bool, error)
	Head() *AclRecord
	Get(id string) (*AclRecord, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)

	AddRawRecord(rawRec *aclrecordproto.RawAclRecordWithId) (added bool, err error)

	Close() (err error)
}

type aclList struct {
	root    *aclrecordproto.RawAclRecordWithId
	records []*AclRecord
	indexes map[string]int
	id      string

	stateBuilder  *aclStateBuilder
	recordBuilder AclRecordBuilder
	aclState      *AclState
	keychain      *keychain.Keychain
	storage       liststorage.ListStorage

	sync.RWMutex
}

func BuildAclListWithIdentity(acc *accountdata.AccountData, storage liststorage.ListStorage) (AclList, error) {
	builder := newAclStateBuilderWithIdentity(acc)
	return build(storage.Id(), builder, newAclRecordBuilder(storage.Id(), crypto.NewKeyStorage()), storage)
}

func BuildAclList(storage liststorage.ListStorage) (AclList, error) {
	return build(storage.Id(), newAclStateBuilder(), newAclRecordBuilder(storage.Id(), crypto.NewKeyStorage()), storage)
}

func build(id string, stateBuilder *aclStateBuilder, recBuilder AclRecordBuilder, storage liststorage.ListStorage) (list AclList, err error) {
	head, err := storage.Head()
	if err != nil {
		return
	}

	rawRecordWithId, err := storage.GetRawRecord(context.Background(), head)
	if err != nil {
		return
	}

	record, err := recBuilder.FromRaw(rawRecordWithId)
	if err != nil {
		return
	}
	records := []*AclRecord{record}

	for record.PrevId != "" {
		rawRecordWithId, err = storage.GetRawRecord(context.Background(), record.PrevId)
		if err != nil {
			return
		}

		record, err = recBuilder.FromRaw(rawRecordWithId)
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

func (a *aclList) Records() []*AclRecord {
	return a.records
}

func (a *aclList) AddRawRecord(rawRec *aclrecordproto.RawAclRecordWithId) (added bool, err error) {
	if _, ok := a.indexes[rawRec.Id]; ok {
		return
	}
	record, err := a.recordBuilder.FromRaw(rawRec)
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

func (a *aclList) IsValidNext(rawRec *aclrecordproto.RawAclRecordWithId) (err error) {
	_, err = a.recordBuilder.FromRaw(rawRec)
	if err != nil {
		return
	}
	// TODO: change state and add "check" method for records
	return
}

func (a *aclList) Id() string {
	return a.id
}

func (a *aclList) Root() *aclrecordproto.RawAclRecordWithId {
	return a.root
}

func (a *aclList) AclState() *AclState {
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

func (a *aclList) Head() *AclRecord {
	return a.records[len(a.records)-1]
}

func (a *aclList) Get(id string) (*AclRecord, error) {
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
