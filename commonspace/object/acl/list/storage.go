package list

import (
	"context"
	"errors"
	"fmt"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/anyproto/any-store/query"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

const (
	orderKey            = "o"
	headsKey            = "h"
	idKey               = "id"
	rawRecordKey        = "r"
	changeSizeKey       = "sz"
	prevIdKey           = "p"
	headsCollectionName = "heads"
)

type StorageRecord struct {
	RawRecord  []byte
	PrevId     string
	Id         string
	Order      int
	ChangeSize int
}

func (c StorageRecord) RawRecordWithId() *consensusproto.RawRecordWithId {
	return &consensusproto.RawRecordWithId{
		Payload: c.RawRecord,
		Id:      c.Id,
	}
}

type StorageIterator = func(ctx context.Context, record StorageRecord) (shouldContinue bool, err error)

type Storage interface {
	Id() string
	Root(ctx context.Context) (StorageRecord, error)
	Head(ctx context.Context) (string, error)
	Has(ctx context.Context, id string) (bool, error)
	Get(ctx context.Context, id string) (StorageRecord, error)
	GetAfterOrder(ctx context.Context, order int, iter StorageIterator) error
	GetBeforeOrder(ctx context.Context, order int, iter StorageIterator) error
	AddAll(ctx context.Context, records []StorageRecord) error
}

type storage struct {
	id          string
	store       anystore.DB
	headStorage headstorage.HeadStorage
	headsColl   anystore.Collection
	recordsColl anystore.Collection
	arena       *anyenc.Arena
}

func CreateStorage(ctx context.Context, root *consensusproto.RawRecordWithId, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {
	tx, err := store.WriteTx(ctx)
	if err != nil {
		return nil, err
	}
	storage, err := CreateStorageTx(tx.Context(), root, headStorage, store)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return storage, tx.Commit()
}

func CreateStorageTx(ctx context.Context, root *consensusproto.RawRecordWithId, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {
	st := &storage{
		id:          root.Id,
		store:       store,
		headStorage: headStorage,
	}
	rec := StorageRecord{
		RawRecord:  root.Payload,
		Id:         root.Id,
		Order:      1,
		ChangeSize: len(root.Payload),
	}
	recordsColl, err := store.Collection(ctx, root.Id)
	if err != nil {
		return nil, err
	}
	st.recordsColl = recordsColl
	orderIdx := anystore.IndexInfo{
		Name:   orderKey,
		Fields: []string{orderKey},
		Unique: true,
	}
	err = st.recordsColl.EnsureIndex(ctx, orderIdx)
	if err != nil {
		return nil, err
	}
	st.arena = &anyenc.Arena{}
	defer st.arena.Reset()
	doc := newStorageRecordValue(rec, st.arena)
	err = st.recordsColl.Insert(ctx, doc)
	if err != nil {
		return nil, err
	}
	err = st.headStorage.UpdateEntry(ctx, headstorage.HeadsUpdate{
		Id:    root.Id,
		Heads: []string{root.Id},
	})
	if err != nil {
		return nil, err
	}
	return st, nil
}

func NewStorage(ctx context.Context, id string, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {
	st := &storage{
		id:          id,
		store:       store,
		headStorage: headStorage,
	}
	recordsColl, err := store.Collection(ctx, id)
	if err != nil {
		return nil, err
	}
	st.recordsColl = recordsColl
	orderIdx := anystore.IndexInfo{
		Name:   orderKey,
		Fields: []string{orderKey},
		Unique: true,
	}
	err = st.recordsColl.EnsureIndex(ctx, orderIdx)
	if err != nil && !errors.Is(err, anystore.ErrIndexExists) {
		return nil, err
	}
	st.arena = &anyenc.Arena{}
	return st, nil
}

func (s *storage) Head(ctx context.Context) (res string, err error) {
	headsEntry, err := s.headStorage.GetEntry(ctx, s.id)
	if err != nil {
		return
	}
	if len(headsEntry.Heads) == 1 {
		return headsEntry.Heads[0], nil
	}
	return "", nil
}

func (s *storage) Has(ctx context.Context, id string) (bool, error) {
	_, err := s.recordsColl.FindId(ctx, id)
	if err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *storage) GetAfterOrder(ctx context.Context, order int, storageIter StorageIterator) error {
	qry := s.recordsColl.Find(query.Key{Path: []string{orderKey}, Filter: query.NewComp(query.CompOpGte, order)}).Sort(orderKey)
	return s.getWithQuery(ctx, qry, storageIter)
}

func (s *storage) GetBeforeOrder(ctx context.Context, order int, storageIter StorageIterator) error {
	qry := s.recordsColl.Find(query.Key{Path: []string{orderKey}, Filter: query.NewComp(query.CompOpLte, order)}).Sort(orderKey)
	return s.getWithQuery(ctx, qry, storageIter)
}

func (s *storage) getWithQuery(ctx context.Context, qry anystore.Query, storageIter StorageIterator) error {
	iter, err := s.recordsColl.Find(qry).Iter(ctx)
	if err != nil {
		return fmt.Errorf("find iter: %w", err)
	}
	defer iter.Close()

	for iter.Next() {
		doc, err := iter.Doc()
		if err != nil {
			return fmt.Errorf("doc not found: %w", err)
		}
		parsed, err := s.changeFromDoc(doc.Value().GetString("id"), doc)
		if err != nil {
			return fmt.Errorf("failed to make change from doc: %w", err)
		}
		cont, err := storageIter(ctx, parsed)
		if !cont {
			return err
		}
	}
	return nil
}

func (s *storage) AddAll(ctx context.Context, records []StorageRecord) error {
	if len(records) == 0 {
		return nil
	}
	arena := s.arena
	defer arena.Reset()
	tx, err := s.store.WriteTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create write tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	vals := make([]*anyenc.Value, 0, len(records))
	for _, ch := range records {
		newVal := newStorageRecordValue(ch, arena)
		vals = append(vals, newVal)
	}
	err = s.recordsColl.Insert(tx.Context(), vals...)
	if err != nil {
		return err
	}
	head := records[len(records)-1].Id
	update := headstorage.HeadsUpdate{
		Id:    s.id,
		Heads: []string{head},
	}
	return s.headStorage.UpdateEntry(tx.Context(), update)
}

func (s *storage) Id() string {
	return s.id
}

func (s *storage) Root(ctx context.Context) (StorageRecord, error) {
	return s.Get(ctx, s.id)
}

func (s *storage) Get(ctx context.Context, id string) (StorageRecord, error) {
	doc, err := s.recordsColl.FindId(ctx, id)
	if err != nil {
		return StorageRecord{}, err
	}
	ch, err := s.changeFromDoc(id, doc)
	if err != nil {
		return StorageRecord{}, err
	}
	return ch, nil
}

func (s *storage) changeFromDoc(id string, doc anystore.Doc) (StorageRecord, error) {
	return StorageRecord{
		Id:         id,
		RawRecord:  doc.Value().GetBytes(rawRecordKey),
		Order:      doc.Value().GetInt(orderKey),
		ChangeSize: doc.Value().GetInt(changeSizeKey),
		PrevId:     doc.Value().GetString(prevIdKey),
	}, nil
}

func newStringArrayValue(strings []string, arena *anyenc.Arena) *anyenc.Value {
	val := arena.NewArray()
	for idx, str := range strings {
		val.SetArrayItem(idx, arena.NewString(str))
	}
	return val
}

func newStorageRecordValue(ch StorageRecord, arena *anyenc.Arena) *anyenc.Value {
	newVal := arena.NewObject()
	newVal.Set(orderKey, arena.NewNumberInt(ch.Order))
	newVal.Set(rawRecordKey, arena.NewBinary(ch.RawRecord))
	newVal.Set(changeSizeKey, arena.NewNumberInt(ch.ChangeSize))
	newVal.Set(idKey, arena.NewString(ch.Id))
	newVal.Set(prevIdKey, arena.NewString(ch.PrevId))
	return newVal
}
