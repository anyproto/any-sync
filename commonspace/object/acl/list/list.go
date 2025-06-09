//go:generate mockgen -destination mock_list/mock_list.go github.com/anyproto/any-sync/commonspace/object/acl/list AclList,Storage
package list

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

type IterFunc = func(record *AclRecord) (IsContinue bool)

var (
	ErrIncorrectCID        = errors.New("incorrect CID")
	ErrRecordAlreadyExists = errors.New("record already exists")
)

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

type AclList interface {
	RWLocker
	Id() string
	Root() *consensusproto.RawRecordWithId
	Records() []*AclRecord
	AclState() *AclState
	IsAfter(first string, second string) (bool, error)
	HasHead(head string) bool
	Head() *AclRecord

	RecordsAfter(ctx context.Context, id string) (records []*consensusproto.RawRecordWithId, err error)
	RecordsBefore(ctx context.Context, headId string) (records []*consensusproto.RawRecordWithId, err error)
	Get(id string) (*AclRecord, error)
	GetIndex(idx int) (*AclRecord, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)

	KeyStorage() crypto.KeyStorage
	RecordBuilder() AclRecordBuilder

	ValidateRawRecord(rawRec *consensusproto.RawRecord, afterValid func(state *AclState) error) (err error)
	AddRawRecord(rawRec *consensusproto.RawRecordWithId) (err error)
	AddRawRecords(rawRecords []*consensusproto.RawRecordWithId) (err error)

	Close(ctx context.Context) (err error)
}

type aclList struct {
	root    *consensusproto.RawRecordWithId
	records []*AclRecord
	indexes map[string]int
	id      string

	stateBuilder  *aclStateBuilder
	recordBuilder AclRecordBuilder
	keyStorage    crypto.KeyStorage
	aclState      *AclState
	storage       Storage
	verifier      recordverifier.AcceptorVerifier

	sync.RWMutex
}

type internalDeps struct {
	storage          Storage
	keyStorage       crypto.KeyStorage
	stateBuilder     *aclStateBuilder
	recordBuilder    AclRecordBuilder
	acceptorVerifier recordverifier.AcceptorVerifier
}

func BuildAclListWithIdentity(acc *accountdata.AccountKeys, storage Storage, verifier recordverifier.AcceptorVerifier) (AclList, error) {
	keyStorage := crypto.NewKeyStorage()
	deps := internalDeps{
		storage:          storage,
		keyStorage:       keyStorage,
		stateBuilder:     newAclStateBuilderWithIdentity(acc),
		recordBuilder:    NewAclRecordBuilder(storage.Id(), keyStorage, acc, verifier),
		acceptorVerifier: verifier,
	}
	return build(deps)
}

func build(deps internalDeps) (list AclList, err error) {
	var (
		ctx          = context.Background()
		storage      = deps.storage
		id           = deps.storage.Id()
		recBuilder   = deps.recordBuilder
		stateBuilder = deps.stateBuilder
	)
	head, err := storage.Head(ctx)
	if err != nil {
		return
	}

	rec, err := storage.Get(ctx, head)
	if err != nil {
		return
	}

	record, err := recBuilder.UnmarshallWithId(rec.RawRecordWithId())
	if err != nil {
		return
	}
	records := []*AclRecord{record}

	for record.PrevId != "" {
		rec, err = storage.Get(ctx, record.PrevId)
		if err != nil {
			return
		}

		record, err = recBuilder.UnmarshallWithId(rec.RawRecordWithId())
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
	// TODO: check if this is correct (raw model instead of unmarshalled)
	rootWithId, err := storage.Root(ctx)
	if err != nil {
		return
	}

	list = &aclList{
		root:          rootWithId.RawRecordWithId(),
		records:       records,
		indexes:       indexes,
		stateBuilder:  stateBuilder,
		recordBuilder: recBuilder,
		storage:       storage,
		verifier:      deps.acceptorVerifier,
		id:            id,
	}
	stateBuilder.Init(id)
	state, err := stateBuilder.Build(records, list.(*aclList))
	if err != nil {
		return
	}
	list.(*aclList).aclState = state
	recBuilder.(*aclRecordBuilder).state = state
	state.list = list.(*aclList)
	return
}

func (a *aclList) RecordBuilder() AclRecordBuilder {
	return a.recordBuilder
}

func (a *aclList) Records() []*AclRecord {
	return a.records
}

func (a *aclList) ValidateRawRecord(rawRec *consensusproto.RawRecord, afterValid func(state *AclState) error) (err error) {
	record, err := a.recordBuilder.Unmarshall(rawRec)
	if err != nil {
		return
	}
	stateCopy := a.aclState.Copy()
	stateCopy.contentValidator = newContentValidator(stateCopy.keyStore, stateCopy, recordverifier.NewValidateFull())
	err = stateCopy.ApplyRecord(record)
	if err != nil || afterValid == nil {
		return
	}
	return afterValid(stateCopy)
}

func (a *aclList) AddRawRecords(rawRecords []*consensusproto.RawRecordWithId) error {
	for _, rec := range rawRecords {
		err := a.AddRawRecord(rec)
		if err != nil && !errors.Is(err, ErrRecordAlreadyExists) {
			return err
		}
	}
	return nil
}

func (a *aclList) AddRawRecord(rawRec *consensusproto.RawRecordWithId) (err error) {
	if _, ok := a.indexes[rawRec.Id]; ok {
		return ErrRecordAlreadyExists
	}
	record, err := a.recordBuilder.UnmarshallWithId(rawRec)
	if err != nil {
		return
	}
	copyState := a.aclState.Copy()
	if err = copyState.ApplyRecord(record); err != nil {
		return
	}
	a.setState(copyState)
	a.records = append(a.records, record)
	a.indexes[record.Id] = len(a.records) - 1
	storageRec := StorageRecord{
		RawRecord:  rawRec.Payload,
		PrevId:     record.PrevId,
		Id:         record.Id,
		Order:      len(a.records),
		ChangeSize: len(rawRec.Payload),
	}
	return a.storage.AddAll(context.Background(), []StorageRecord{storageRec})
}

func (a *aclList) setState(state *AclState) {
	a.aclState = state
	a.recordBuilder.(*aclRecordBuilder).state = state
}

func (a *aclList) Id() string {
	return a.id
}

func (a *aclList) Root() *consensusproto.RawRecordWithId {
	return a.root
}

func (a *aclList) AclState() *AclState {
	return a.aclState
}

func (a *aclList) KeyStorage() crypto.KeyStorage {
	return a.keyStorage
}

func (a *aclList) IsAfter(first string, second string) (bool, error) {
	firstRec, okFirst := a.indexes[first]
	secondRec, okSecond := a.indexes[second]
	if !okFirst || !okSecond {
		return false, fmt.Errorf("not all entries are there: first (%t), second (%t)", okFirst, okSecond)
	}
	return firstRec >= secondRec, nil
}

func (a *aclList) isAfterNoCheck(first, second string) bool {
	return a.indexes[first] >= a.indexes[second]
}

func (a *aclList) Head() *AclRecord {
	return a.records[len(a.records)-1]
}

func (a *aclList) HasHead(head string) bool {
	_, exists := a.indexes[head]
	return exists
}

func (a *aclList) Get(id string) (*AclRecord, error) {
	recIdx, ok := a.indexes[id]
	if !ok {
		return nil, ErrNoSuchRecord
	}
	return a.records[recIdx], nil
}

func (a *aclList) GetIndex(idx int) (*AclRecord, error) {
	// TODO: when we add snapshots we will have to monitor record num in snapshots
	if idx < 0 || idx >= len(a.records) {
		return nil, ErrNoSuchRecord
	}
	return a.records[idx], nil
}

func (a *aclList) Iterate(iterFunc IterFunc) {
	for _, rec := range a.records {
		if !iterFunc(rec) {
			return
		}
	}
}

func (a *aclList) RecordsAfter(ctx context.Context, id string) (records []*consensusproto.RawRecordWithId, err error) {
	var recIdx int
	if id == "" {
		recIdx = 1
	} else {
		var ok bool
		recIdx, ok = a.indexes[id]
		if !ok {
			return nil, ErrNoSuchRecord
		}
	}
	err = a.storage.GetAfterOrder(ctx, recIdx, func(ctx context.Context, record StorageRecord) (shouldContinue bool, err error) {
		raw := make([]byte, 0, len(record.RawRecord))
		raw = append(raw, record.RawRecord...)
		records = append(records, &consensusproto.RawRecordWithId{
			Payload: raw,
			Id:      record.Id,
		})
		return true, nil
	})
	return
}

func (a *aclList) RecordsBefore(ctx context.Context, headId string) (records []*consensusproto.RawRecordWithId, err error) {
	if headId == "" {
		headId = a.Head().Id
	}
	recIdx, ok := a.indexes[headId]
	if !ok {
		return nil, ErrNoSuchRecord
	}
	err = a.storage.GetBeforeOrder(ctx, recIdx, func(ctx context.Context, record StorageRecord) (shouldContinue bool, err error) {
		raw := make([]byte, 0, len(record.RawRecord))
		raw = append(raw, record.RawRecord...)
		records = append(records, &consensusproto.RawRecordWithId{
			Payload: raw,
			Id:      record.Id,
		})
		return true, nil
	})
	return
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

func (a *aclList) Close(ctx context.Context) (err error) {
	return nil
}

func WrapAclRecord(rawRec *consensusproto.RawRecord) *consensusproto.RawRecordWithId {
	payload, err := rawRec.Marshal()
	if err != nil {
		panic(err)
	}
	id, err := cidutil.NewCidFromBytes(payload)
	if err != nil {
		panic(err)
	}
	return &consensusproto.RawRecordWithId{
		Payload: payload,
		Id:      id,
	}
}
