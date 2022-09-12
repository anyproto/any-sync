package list

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
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
	Header() *aclpb.ACLHeader
	Records() []*ACLRecord
	ACLState() *ACLState
	IsAfter(first string, second string) (bool, error)
	Head() *ACLRecord
	Get(id string) (*ACLRecord, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)
	Close() (err error)
}

type aclList struct {
	header  *aclpb.ACLHeader
	records []*ACLRecord
	indexes map[string]int
	id      string

	builder  *aclStateBuilder
	aclState *ACLState
	keychain *common.Keychain

	sync.RWMutex
}

func BuildACLListWithIdentity(acc *account.AccountData, storage storage.ListStorage) (ACLList, error) {
	builder := newACLStateBuilderWithIdentity(acc.Decoder, acc)
	return buildWithACLStateBuilder(builder, storage)
}

func BuildACLList(decoder keys.Decoder, storage storage.ListStorage) (ACLList, error) {
	return buildWithACLStateBuilder(newACLStateBuilder(decoder), storage)
}

func buildWithACLStateBuilder(builder *aclStateBuilder, storage storage.ListStorage) (list ACLList, err error) {
	header, err := storage.Header()
	if err != nil {
		return
	}

	id, err := storage.ID()
	if err != nil {
		return
	}

	rawRecord, err := storage.Head()
	if err != nil {
		return
	}

	keychain := common.NewKeychain()
	record, err := NewFromRawRecord(rawRecord)
	if err != nil {
		return
	}
	err = verifyRecord(keychain, rawRecord, record)
	if err != nil {
		return
	}
	records := []*ACLRecord{record}

	for record.Content.PrevId != "" {
		rawRecord, err = storage.GetRawRecord(context.Background(), record.Content.PrevId)
		if err != nil {
			return
		}

		record, err = NewFromRawRecord(rawRecord)
		if err != nil {
			return
		}
		err = verifyRecord(keychain, rawRecord, record)
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

	state, err := builder.Build(records)
	if err != nil {
		return
	}

	list = &aclList{
		header:   header,
		records:  records,
		indexes:  indexes,
		builder:  builder,
		aclState: state,
		id:       id,
		RWMutex:  sync.RWMutex{},
	}
	return
}

func (a *aclList) Records() []*ACLRecord {
	return a.records
}

func (a *aclList) ID() string {
	return a.id
}

func (a *aclList) Header() *aclpb.ACLHeader {
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

func verifyRecord(keychain *common.Keychain, rawRecord *aclpb.RawACLRecord, record *ACLRecord) (err error) {
	identityKey, err := keychain.GetOrAdd(record.Identity)
	if err != nil {
		return
	}

	// verifying signature
	res, err := identityKey.Verify(rawRecord.Payload, rawRecord.Signature)
	if err != nil {
		return
	}
	if !res {
		err = ErrInvalidSignature
		return
	}

	// verifying ID
	if !cid.VerifyCID(rawRecord.Payload, rawRecord.Id) {
		err = ErrIncorrectCID
	}
	return
}
