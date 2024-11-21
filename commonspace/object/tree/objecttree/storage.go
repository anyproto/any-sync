package objecttree

import (
	"context"
	"errors"
	"fmt"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"github.com/anyproto/any-store/query"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/crypto"
)

const (
	orderKey              = "o"
	headsKey              = "c"
	commonSnapshotKey     = "s"
	idKey                 = "id"
	rawChangeKey          = "r"
	snapshotCounterKey    = "sc"
	changeSizeKey         = "sz"
	snapshotIdKey         = "i"
	prevIdsKey            = "p"
	headsCollectionName   = "heads"
	changesCollectionName = "changes"
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

type StorageIterator = func(ctx context.Context, change StorageChange) (shouldContinue bool)

type Storage interface {
	Id() string
	Root(ctx context.Context) (StorageChange, error)
	Heads(ctx context.Context) ([]string, error)
	CommonSnapshot(ctx context.Context) (string, error)
	Has(ctx context.Context, id string) (bool, error)
	Get(ctx context.Context, id string) (StorageChange, error)
	GetAfterOrder(ctx context.Context, orderId string, iter StorageIterator) error
	AddAll(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error
	Delete() error
}

type storage struct {
	id          string
	store       anystore.DB
	headsColl   anystore.Collection
	changesColl anystore.Collection
	arena       *anyenc.Arena
}

func createStorage(ctx context.Context, root *treechangeproto.RawTreeChangeWithId, store anystore.DB) (Storage, error) {
	st := &storage{
		id:    root.Id,
		store: store,
	}
	builder := NewChangeBuilder(crypto.NewKeyStorage(), root)
	_, err := builder.Unmarshall(root, true)
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
	orderIdx := anystore.IndexInfo{
		Name:   orderKey,
		Fields: []string{orderKey},
		Unique: true,
	}
	err = st.changesColl.EnsureIndex(ctx, orderIdx)
	if err != nil {
		return nil, err
	}
	headsColl, err := store.Collection(ctx, headsCollectionName)
	if err != nil {
		return nil, err
	}
	st.headsColl = headsColl
	changesColl, err := store.Collection(ctx, changesCollectionName)
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
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
	headsDoc := st.arena.NewObject()
	headsDoc.Set(headsKey, newStringArrayValue([]string{root.Id}, st.arena))
	headsDoc.Set(commonSnapshotKey, st.arena.NewString(root.Id))
	err = st.headsColl.Insert(tx.Context(), doc)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return st, nil
}

func newStorage(ctx context.Context, id string, store anystore.DB) (Storage, error) {
	// TODO: use spacestorage to set heads
	st := &storage{
		id:    id,
		store: store,
	}
	orderIdx := anystore.IndexInfo{
		Name:   orderKey,
		Fields: []string{orderKey},
		Unique: true,
	}
	err := st.changesColl.EnsureIndex(ctx, orderIdx)
	if err != nil {
		return nil, err
	}
	headsColl, err := store.Collection(ctx, headsCollectionName)
	if err != nil {
		return nil, err
	}
	st.headsColl = headsColl
	changesColl, err := store.Collection(ctx, changesCollectionName)
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
	st.arena = &anyenc.Arena{}
	return st, nil
}

func (s *storage) Heads(ctx context.Context) (res []string, err error) {
	doc, err := s.headsColl.FindId(ctx, s.id)
	if err != nil {
		return nil, err
	}
	res = make([]string, 0, len(res))
	heads := doc.Value().GetArray(headsKey)
	for _, h := range heads {
		res = append(res, h.GetString())
	}
	return
}

func (s *storage) Has(ctx context.Context, id string) (bool, error) {
	_, err := s.headsColl.FindId(ctx, s.id)
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
		parsed, err := s.changeFromDoc(doc.Value().GetString("id"), doc)
		if err != nil {
			return fmt.Errorf("failed to make change from doc: %w", err)
		}
		cont := storageIter(ctx, parsed)
		if !cont {
			break
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
	mod := query.ModifyFunc(func(a *anyenc.Arena, v *anyenc.Value) (result *anyenc.Value, modified bool, err error) {
		v.Set(headsKey, newStringArrayValue(heads, a))
		v.Set(commonSnapshotKey, a.NewString(commonSnapshot))
		return v, true, nil
	})
	_, err = s.headsColl.UpsertId(tx.Context(), s.id, mod)
	if err != nil {
		tx.Rollback()
		return nil
	}
	return tx.Commit()
}

func (s *storage) Delete() error {
	// TODO: add context
	tx, err := s.store.WriteTx(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create write tx: %w", err)
	}
	err = s.changesColl.Drop(tx.Context())
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete changes collection: %w", err)
	}
	err = s.headsColl.DeleteId(tx.Context(), s.id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove document from heads collection: %w", err)
	}
	return nil
}

func (s *storage) Id() string {
	return s.id
}

func (s *storage) Root(ctx context.Context) (StorageChange, error) {
	return s.Get(ctx, s.id)
}

func (s *storage) CommonSnapshot(ctx context.Context) (string, error) {
	// TODO: cache this in memory if needed
	doc, err := s.headsColl.FindId(ctx, s.id)
	if err != nil {
		return "", err
	}
	return doc.Value().GetString(commonSnapshotKey), nil
}

func (s *storage) Get(ctx context.Context, id string) (StorageChange, error) {
	doc, err := s.changesColl.FindId(ctx, id)
	if err != nil {
		return StorageChange{}, err
	}
	ch, err := s.changeFromDoc(s.id, doc)
	if err != nil {
		return StorageChange{}, err
	}
	return ch, nil
}

func (s *storage) changeFromDoc(id string, doc anystore.Doc) (StorageChange, error) {
	change := StorageChange{
		Id:              id,
		RawChange:       doc.Value().GetBytes(rawChangeKey),
		SnapshotId:      doc.Value().GetString(snapshotIdKey),
		OrderId:         doc.Value().GetString(orderKey),
		ChangeSize:      doc.Value().GetInt(changeSizeKey),
		SnapshotCounter: doc.Value().GetInt(snapshotCounterKey),
	}
	prevIds := doc.Value().GetArray(prevIdsKey)
	change.PrevIds = make([]string, 0, len(prevIds))
	for _, item := range prevIds {
		change.PrevIds = append(change.PrevIds, item.GetString())
	}
	return change, nil
}

func newStringArrayValue(strings []string, arena *anyenc.Arena) *anyenc.Value {
	val := arena.NewArray()
	for idx, str := range strings {
		val.SetArrayItem(idx, arena.NewString(str))
	}
	return val
}

func newStorageChangeValue(ch StorageChange, arena *anyenc.Arena) *anyenc.Value {
	newVal := arena.NewObject()
	newVal.Set(orderKey, arena.NewString(ch.OrderId))
	newVal.Set(rawChangeKey, arena.NewBinary(ch.RawChange))
	newVal.Set(snapshotCounterKey, arena.NewNumberInt(ch.SnapshotCounter))
	newVal.Set(snapshotIdKey, arena.NewString(ch.SnapshotId))
	newVal.Set(changeSizeKey, arena.NewNumberInt(ch.ChangeSize))
	if len(ch.PrevIds) != 0 {
		newVal.Set(prevIdsKey, newStringArrayValue(ch.PrevIds, arena))
	}
	return newVal
}
