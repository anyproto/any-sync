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
	OrderKey           = "o"
	idKey              = "id"
	rawChangeKey       = "r"
	SnapshotCounterKey = "sc"
	ChangeSizeKey      = "sz"
	snapshotIdKey      = "i"
	addedKey           = "a"
	prevIdsKey         = "p"
	TreeKey            = "t"
	CollName           = "changes"
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
	AddAllNoError(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error
	Delete(ctx context.Context) error
	Close() error
}

type storage struct {
	id          string
	store       anystore.DB
	headStorage headstorage.HeadStorage
	changesColl anystore.Collection
	arena       *anyenc.Arena
	parser      *anyenc.Parser
	root        StorageChange
}

var StorageChangeBuilder = NewChangeBuilder

func CreateStorage(ctx context.Context, root *treechangeproto.RawTreeChangeWithId, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {
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

func CreateStorageTx(ctx context.Context, root *treechangeproto.RawTreeChangeWithId, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {
	st := &storage{
		id:          root.Id,
		store:       store,
		headStorage: headStorage,
	}
	builder := StorageChangeBuilder(crypto.NewKeyStorage(), root)
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
	changesColl, err := store.Collection(ctx, CollName)
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
	st.arena = &anyenc.Arena{}
	st.parser = &anyenc.Parser{}
	defer st.arena.Reset()
	doc := newStorageChangeValue(stChange, st.arena)
	err = st.changesColl.Insert(ctx, doc)
	if err != nil {
		if errors.Is(err, anystore.ErrDocExists) {
			return nil, treestorage.ErrTreeExists
		}
		return nil, err
	}
	err = st.headStorage.UpdateEntryTx(ctx, headstorage.HeadsUpdate{
		Id:             root.Id,
		Heads:          []string{root.Id},
		CommonSnapshot: &root.Id,
		IsDerived:      &unmarshalled.IsDerived,
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
	if _, err := headStorage.GetEntry(ctx, id); err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return nil, treestorage.ErrUnknownTreeId
		}
		return nil, fmt.Errorf("failed to get head entry: %w", err)
	}
	changesColl, err := store.OpenCollection(ctx, CollName)
	if err != nil {
		return nil, fmt.Errorf("failed to open collection: %w", err)
	}
	st.changesColl = changesColl
	st.arena = &anyenc.Arena{}
	st.parser = &anyenc.Parser{}
	st.root, err = st.getWithoutParser(ctx, st.id)
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
		err = fmt.Errorf("failed to get heads entry: %w", err)
		return
	}
	return headsEntry.Heads, nil
}

func (s *storage) Has(ctx context.Context, id string) (bool, error) {
	_, err := s.changesColl.FindIdWithParser(ctx, s.parser, id)
	if err != nil {
		if errors.Is(err, anystore.ErrDocNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *storage) GetAfterOrder(ctx context.Context, orderId string, storageIter StorageIterator) error {
	// this method can be called without having a lock on a tree, so don't reuse any non-thread-safe parts
	filter := query.And{
		query.Key{Path: []string{OrderKey}, Filter: query.NewComp(query.CompOpGte, orderId)},
		query.Key{Path: []string{TreeKey}, Filter: query.NewComp(query.CompOpEq, s.id)},
	}
	qry := s.changesColl.Find(filter).Sort(OrderKey)
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
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	for _, ch := range changes {
		ch.TreeId = s.id
		newVal := newStorageChangeValue(ch, arena)
		err = s.changesColl.Insert(tx.Context(), newVal)
		arena.Reset()
		if err != nil {
			return err
		}
	}
	update := headstorage.HeadsUpdate{
		Id:             s.id,
		Heads:          heads,
		CommonSnapshot: &commonSnapshot,
	}
	return s.headStorage.UpdateEntryTx(tx.Context(), update)
}

func (s *storage) AddAllNoError(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error {
	arena := s.arena
	defer arena.Reset()
	tx, err := s.store.WriteTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create write tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	for _, ch := range changes {
		ch.TreeId = s.id
		newVal := newStorageChangeValue(ch, arena)
		err = s.changesColl.Insert(tx.Context(), newVal)
		arena.Reset()
		if err != nil && !errors.Is(err, anystore.ErrDocExists) {
			return err
		}
	}
	update := headstorage.HeadsUpdate{
		Id:             s.id,
		Heads:          heads,
		CommonSnapshot: &commonSnapshot,
	}
	return s.headStorage.UpdateEntryTx(tx.Context(), update)
}

func (s *storage) Delete(ctx context.Context) error {
	tx, err := s.store.WriteTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create write tx: %w", err)
	}
	_, err = s.changesColl.Find(query.Key{Path: []string{TreeKey}, Filter: query.NewComp(query.CompOpEq, s.id)}).Delete(tx.Context())
	if err != nil {
		tx.Rollback()
		return err
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
		return "", fmt.Errorf("failed to get head entry for common snapshot: %w", err)
	}
	return entry.CommonSnapshot, nil
}

func (s *storage) getWithoutParser(ctx context.Context, id string) (StorageChange, error) {
	// root will be reused outside the lock, so we shouldn't use parser for it
	doc, err := s.changesColl.FindId(ctx, id)
	if err != nil {
		return StorageChange{}, err
	}
	return s.changeFromDoc(doc), nil
}

func (s *storage) Get(ctx context.Context, id string) (StorageChange, error) {
	doc, err := s.changesColl.FindIdWithParser(ctx, s.parser, id)
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
		OrderId:         doc.Value().GetString(OrderKey),
		ChangeSize:      doc.Value().GetInt(ChangeSizeKey),
		SnapshotCounter: doc.Value().GetInt(SnapshotCounterKey),
		PrevIds:         storeutil.StringsFromArrayValue(doc.Value(), prevIdsKey),
	}
}

func newStorageChangeValue(ch StorageChange, arena *anyenc.Arena) *anyenc.Value {
	newVal := arena.NewObject()
	newVal.Set(OrderKey, arena.NewString(ch.OrderId))
	newVal.Set(rawChangeKey, arena.NewBinary(ch.RawChange))
	newVal.Set(SnapshotCounterKey, arena.NewNumberInt(ch.SnapshotCounter))
	newVal.Set(snapshotIdKey, arena.NewString(ch.SnapshotId))
	newVal.Set(ChangeSizeKey, arena.NewNumberInt(ch.ChangeSize))
	newVal.Set(idKey, arena.NewString(ch.Id))
	newVal.Set(TreeKey, arena.NewString(ch.TreeId))
	newVal.Set(addedKey, arena.NewNumberFloat64(float64(time.Now().Unix())))
	if len(ch.PrevIds) != 0 {
		newVal.Set(prevIdsKey, storeutil.NewStringArrayValue(ch.PrevIds, arena))
	}
	return newVal
}
