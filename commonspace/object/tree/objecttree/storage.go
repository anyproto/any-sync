package objecttree

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/anyproto/any-store/query"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/storeutil"
)

const (
	orderKey           = "o"
	idKey              = "id"
	rawChangeKey       = "r"
	snapshotCounterKey = "sc"
	changeSizeKey      = "sz"
	snapshotIdKey      = "i"
	addedKey           = "a"
	prevIdsKey         = "p"
	treeKey            = "t"
	collName           = "changes"
)

type StorageChange struct {
	RawChange       []byte
	PrevIds         []string
	Id              string
	SnapshotCounter int
	SnapshotId      string
	OrderId         string
	ChangeSize      int
	TreeId          string
}

func (c StorageChange) RawTreeChangeWithId() *treechangeproto.RawTreeChangeWithId {
	return &treechangeproto.RawTreeChangeWithId{
		RawChange: c.RawChange,
		Id:        c.Id,
	}
}

type StorageIterator = func(ctx context.Context, change StorageChange) (shouldContinue bool, err error)

type Storage interface {
	Id() string
	Root(ctx context.Context) (StorageChange, error)
	Heads(ctx context.Context) ([]string, error)
	CommonSnapshot(ctx context.Context) (string, error)
	Has(ctx context.Context, id string) (bool, error)
	Get(ctx context.Context, id string) (StorageChange, error)
	GetAfterOrder(ctx context.Context, orderId string, iter StorageIterator) error
	AddAll(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error
	Delete(ctx context.Context) error
	Close() error
}

type storage struct {
	id          string
	store       anystore.DB
	headStorage headstorage.HeadStorage
	changesColl anystore.Collection
	arena       *anyenc.Arena
	root        StorageChange
}

var storageChangeBuilder = NewChangeBuilder

func CreateStorage(ctx context.Context, root *treechangeproto.RawTreeChangeWithId, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {
	st := &storage{
		id:          root.Id,
		store:       store,
		headStorage: headStorage,
	}
	builder := storageChangeBuilder(crypto.NewKeyStorage(), root)
	unmarshalled, err := builder.Unmarshall(root, true)
	if err != nil {
		return nil, err
	}
	firstOrder := lexId.Next("")
	stChange := StorageChange{
		RawChange:       root.RawChange,
		Id:              root.Id,
		SnapshotCounter: 1,
		SnapshotId:      "",
		OrderId:         firstOrder,
		TreeId:          root.Id,
		ChangeSize:      len(root.RawChange),
	}
	st.root = stChange
	changesColl, err := store.Collection(ctx, collName)
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
	orderIdx := anystore.IndexInfo{
		Fields: []string{treeKey, orderKey},
		Unique: true,
	}
	err = st.changesColl.EnsureIndex(ctx, orderIdx)
	if err != nil {
		return nil, err
	}
	st.arena = &anyenc.Arena{}
	defer st.arena.Reset()
	doc := newStorageChangeValue(stChange, st.arena)
	tx, err := st.store.WriteTx(ctx)
	if err != nil {
		return nil, err
	}
	err = st.changesColl.Insert(tx.Context(), doc)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, anystore.ErrDocExists) {
			return nil, treestorage.ErrTreeExists
		}
		return nil, err
	}
	err = st.headStorage.UpdateEntryTx(tx.Context(), headstorage.HeadsUpdate{
		Id:             root.Id,
		Heads:          []string{root.Id},
		CommonSnapshot: &root.Id,
		IsDerived:      &unmarshalled.IsDerived,
	})
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return st, tx.Commit()
}

func NewStorage(ctx context.Context, id string, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {
	st := &storage{
		id:          id,
		store:       store,
		headStorage: headStorage,
	}
	if _, err := headStorage.GetEntry(ctx, id); err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return nil, treestorage.ErrUnknownTreeId
		}
		return nil, err
	}
	changesColl, err := store.OpenCollection(ctx, collName)
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
	st.arena = &anyenc.Arena{}
	st.root, err = st.Get(ctx, st.id)
	if err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return nil, treestorage.ErrUnknownTreeId
		}
		return nil, err
	}
	return st, nil
}

func (s *storage) Heads(ctx context.Context) (res []string, err error) {
	headsEntry, err := s.headStorage.GetEntry(ctx, s.id)
	if err != nil {
		return
	}
	return headsEntry.Heads, nil
}

func (s *storage) Has(ctx context.Context, id string) (bool, error) {
	_, err := s.changesColl.FindId(ctx, id)
	if err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *storage) GetAfterOrder(ctx context.Context, orderId string, storageIter StorageIterator) error {
	filter := query.And{
		query.Key{Path: []string{orderKey}, Filter: query.NewComp(query.CompOpGte, orderId)},
		query.Key{Path: []string{treeKey}, Filter: query.NewComp(query.CompOpEq, s.id)},
	}
	qry := s.changesColl.Find(filter).Sort(orderKey)
	iter, err := qry.Iter(ctx)
	if err != nil {
		return fmt.Errorf("find iter: %w", err)
	}
	defer iter.Close()

	for iter.Next() {
		doc, err := iter.Doc()
		if err != nil {
			return fmt.Errorf("doc not found: %w", err)
		}
		cont, err := storageIter(ctx, s.changeFromDoc(doc))
		if !cont {
			return err
		}
	}
	return nil
}

func (s *storage) AddAll(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error {
	arena := s.arena
	defer arena.Reset()
	tx, err := s.store.WriteTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create write tx: %w", err)
	}
	vals := make([]*anyenc.Value, 0, len(changes))
	for _, ch := range changes {
		ch.TreeId = s.id
		newVal := newStorageChangeValue(ch, arena)
		vals = append(vals, newVal)
	}
	err = s.changesColl.Insert(tx.Context(), vals...)
	if err != nil {
		tx.Rollback()
		return nil
	}
	update := headstorage.HeadsUpdate{
		Id:             s.id,
		Heads:          heads,
		CommonSnapshot: &commonSnapshot,
	}
	err = s.headStorage.UpdateEntryTx(tx.Context(), update)
	if err != nil {
		tx.Rollback()
		return nil
	}
	return tx.Commit()
}

func (s *storage) Delete(ctx context.Context) error {
	tx, err := s.store.WriteTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create write tx: %w", err)
	}
	err = s.changesColl.Drop(tx.Context())
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete changes collection: %w", err)
	}
	return tx.Commit()
}

func (s *storage) Close() error {
	return s.changesColl.Close()
}

func (s *storage) Id() string {
	return s.id
}

func (s *storage) Root(ctx context.Context) (StorageChange, error) {
	return s.root, nil
}

func (s *storage) CommonSnapshot(ctx context.Context) (string, error) {
	// TODO: cache this in memory if needed
	entry, err := s.headStorage.GetEntry(ctx, s.id)
	if err != nil {
		return "", err
	}
	return entry.CommonSnapshot, nil
}

var (
	totalCalls atomic.Int32
	totalTime  atomic.Int64
)

func (s *storage) Get(ctx context.Context, id string) (StorageChange, error) {
	tm := time.Now()
	totalCalls.Add(1)
	doc, err := s.changesColl.FindId(ctx, id)
	totalTime.Add(int64(time.Since(tm)))
	if totalCalls.Load()%100 == 0 {
		fmt.Println("[x]: totalTime", time.Duration(totalTime.Load()).String(), "totalCalls", totalCalls.Load())
	}
	if err != nil {
		return StorageChange{}, err
	}
	return s.changeFromDoc(doc), nil
}

func (s *storage) changeFromDoc(doc anystore.Doc) StorageChange {
	return StorageChange{
		Id:              doc.Value().GetString(idKey),
		RawChange:       doc.Value().GetBytes(rawChangeKey),
		SnapshotId:      doc.Value().GetString(snapshotIdKey),
		OrderId:         doc.Value().GetString(orderKey),
		ChangeSize:      doc.Value().GetInt(changeSizeKey),
		SnapshotCounter: doc.Value().GetInt(snapshotCounterKey),
		PrevIds:         storeutil.StringsFromArrayValue(doc.Value(), prevIdsKey),
	}
}

func newStorageChangeValue(ch StorageChange, arena *anyenc.Arena) *anyenc.Value {
	newVal := arena.NewObject()
	newVal.Set(orderKey, arena.NewString(ch.OrderId))
	newVal.Set(rawChangeKey, arena.NewBinary(ch.RawChange))
	newVal.Set(snapshotCounterKey, arena.NewNumberInt(ch.SnapshotCounter))
	newVal.Set(snapshotIdKey, arena.NewString(ch.SnapshotId))
	newVal.Set(changeSizeKey, arena.NewNumberInt(ch.ChangeSize))
	newVal.Set(idKey, arena.NewString(ch.Id))
	newVal.Set(treeKey, arena.NewString(ch.TreeId))
	newVal.Set(addedKey, arena.NewNumberFloat64(float64(time.Now().Unix())))
	if len(ch.PrevIds) != 0 {
		newVal.Set(prevIdsKey, storeutil.NewStringArrayValue(ch.PrevIds, arena))
	}
	return newVal
}
