package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
)

type IterFunc = func(record *Record) (IsContinue bool)

type ACLList interface {
	tree.RWLocker
	ID() string
	Header() *aclpb.Header
	ACLState() ACLState
	IsAfter(first string, second string) (bool, error)
	Head() *Record
	Get(id string) (*Record, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)
}

//func (t *ACLListStorageBuilder) IsAfter(first string, second string) (bool, error) {
//	firstRec, okFirst := t.indexes[first]
//	secondRec, okSecond := t.indexes[second]
//	if !okFirst || !okSecond {
//		return false, fmt.Errorf("not all entries are there: first (%b), second (%b)", okFirst, okSecond)
//	}
//	return firstRec > secondRec, nil
//}
//
//func (t *ACLListStorageBuilder) Head() *list.Record {
//	return t.records[len(t.records)-1]
//}
//
//func (t *ACLListStorageBuilder) Get(id string) (*list.Record, error) {
//	recIdx, ok := t.indexes[id]
//	if !ok {
//		return nil, fmt.Errorf("no such record")
//	}
//	return t.records[recIdx], nil
//}
//
//func (t *ACLListStorageBuilder) Iterate(iterFunc list.IterFunc) {
//	for _, rec := range t.records {
//		if !iterFunc(rec) {
//			return
//		}
//	}
//}
//
//func (t *ACLListStorageBuilder) IterateFrom(startId string, iterFunc list.IterFunc) {
//	recIdx, ok := t.indexes[startId]
//	if !ok {
//		return
//	}
//	for i := recIdx; i < len(t.records); i++ {
//		if !iterFunc(t.records[i]) {
//			return
//		}
//	}
//}
