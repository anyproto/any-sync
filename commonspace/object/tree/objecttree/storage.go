package objecttree

import (
	"context"
	"errors"
	"fmt"
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
)

type StorageChange struct {
	RawChange       []byte
	PrevIds         []string
	Id              string
	SnapshotCounter int
	SnapshotId      string
	OrderId         string
	ChangeSize      int
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
		ChangeSize:      len(root.RawChange),
	}
	changesColl, err := store.Collection(ctx, root.Id)
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
	orderIdx := anystore.IndexInfo{
		Name:   orderKey,
		Fields: []string{orderKey},
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
	entry, err := headStorage.GetEntry(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get heads entry for storage, %w, %s: %w", treestorage.ErrUnknownTreeId, id, err)
	}
	if len(entry.Heads) == 0 {
		return nil, fmt.Errorf("no heads found for storage %w: %s", treestorage.ErrUnknownTreeId, id)
	}
	st := &storage{
		id:          id,
		store:       store,
		headStorage: headStorage,
	}
	changesColl, err := store.Collection(ctx, id)
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
	orderIdx := anystore.IndexInfo{
		Name:   orderKey,
		Fields: []string{orderKey},
		Unique: true,
	}
	err = st.changesColl.EnsureIndex(ctx, orderIdx)
	if err != nil && !errors.Is(err, anystore.ErrIndexExists) {
		return nil, err
	}
	st.arena = &anyenc.Arena{}
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
	qry := s.changesColl.Find(query.Key{Path: []string{orderKey}, Filter: query.NewComp(query.CompOpGte, orderId)}).Sort(orderKey)
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
	return s.Get(ctx, s.id)
}

func (s *storage) CommonSnapshot(ctx context.Context) (string, error) {
	// TODO: cache this in memory if needed
	entry, err := s.headStorage.GetEntry(ctx, s.id)
	if err != nil {
		return "", err
	}
	return entry.CommonSnapshot, nil
}

func (s *storage) Get(ctx context.Context, id string) (StorageChange, error) {
	doc, err := s.changesColl.FindId(ctx, id)
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
	newVal.Set(addedKey, arena.NewNumberFloat64(float64(time.Now().Unix())))
	if len(ch.PrevIds) != 0 {
		newVal.Set(prevIdsKey, storeutil.NewStringArrayValue(ch.PrevIds, arena))
	}
	return newVal
}
