package list

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"go.uber.org/zap"
	"sync"
)

type IterFunc = func(record *Record) (IsContinue bool)

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

type ACLList interface {
	RWLocker
	ID() string
	Header() *aclpb.Header
	Records() []*Record
	ACLState() *ACLState
	IsAfter(first string, second string) (bool, error)
	Head() *Record
	Get(id string) (*Record, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)
	Close() (err error)
}

type aclList struct {
	header  *aclpb.Header
	records []*Record
	indexes map[string]int
	id      string

	builder  *aclStateBuilder
	aclState *ACLState

	sync.RWMutex
}

func BuildACLListWithIdentity(acc *account.AccountData, storage storage.ListStorage) (ACLList, error) {
	builder := newACLStateBuilderWithIdentity(acc.Decoder, acc)
	return buildWithACLStateBuilder(builder, storage)
}

func BuildACLList(decoder keys.Decoder, storage storage.ListStorage) (ACLList, error) {
	return buildWithACLStateBuilder(newACLStateBuilder(decoder), storage)
}

func buildWithACLStateBuilder(builder *aclStateBuilder, storage storage.ListStorage) (ACLList, error) {
	header, err := storage.Header()
	if err != nil {
		return nil, err
	}

	id, err := storage.ID()
	if err != nil {
		return nil, err
	}

	rawRecord, err := storage.Head()
	if err != nil {
		return nil, err
	}

	record, err := NewFromRawRecord(rawRecord)
	if err != nil {
		return nil, err
	}
	records := []*Record{record}

	for record.Content.PrevId != "" {
		rawRecord, err = storage.GetRawRecord(context.Background(), record.Content.PrevId)
		if err != nil {
			return nil, err
		}
		record, err = NewFromRawRecord(rawRecord)
		if err != nil {
			return nil, err
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

	log.With(zap.String("head id", records[len(records)-1].Id), zap.String("list id", id)).
		Info("building acl tree")
	state, err := builder.Build(records)
	if err != nil {
		return nil, err
	}

	return &aclList{
		header:   header,
		records:  records,
		indexes:  indexes,
		builder:  builder,
		aclState: state,
		id:       id,
		RWMutex:  sync.RWMutex{},
	}, nil
}

func (a *aclList) Records() []*Record {
	return a.records
}

func (a *aclList) ID() string {
	return a.id
}

func (a *aclList) Header() *aclpb.Header {
	return a.header
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

func (a *aclList) Head() *Record {
	return a.records[len(a.records)-1]
}

func (a *aclList) Get(id string) (*Record, error) {
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
