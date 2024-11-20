package objecttree

import (
	"context"

	anystore "github.com/anyproto/any-store"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
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
}

func newStorage(ctx context.Context, id string, store anystore.DB) (Storage, error) {
	st := &storage{
		id:    id,
		store: store,
	}
	orderIdx := anystore.IndexInfo{
		Name:   "order",
		Fields: []string{"order"},
		Unique: true,
	}
	err := st.changesColl.EnsureIndex(ctx, orderIdx)
	if err != nil {
		return nil, err
	}
	headsColl, err := store.Collection(ctx, "heads")
	if err != nil {
		return nil, err
	}
	st.headsColl = headsColl
	changesColl, err := store.Collection(ctx, "changes")
	if err != nil {
		return nil, err
	}
	st.changesColl = changesColl
	return st, nil
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
	return doc.Value().GetString("commonSnapshot"), nil
}

func (s *storage) Heads(ctx context.Context) (string, error) {
	doc, err := s.headsColl.FindId(ctx, s.id)
	if err != nil {
		return "", err
	}
	return doc.Value().GetString("commonSnapshot"), nil
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
		RawChange:       doc.Value().GetBytes("rawChange"),
		SnapshotId:      doc.Value().GetString("snapshotId"),
		OrderId:         doc.Value().GetString("orderId"),
		ChangeSize:      doc.Value().GetInt("changeSize"),
		SnapshotCounter: doc.Value().GetInt("snapshotCounter"),
	}
	prevIds := doc.Value().GetArray("prevIds")
	change.PrevIds = make([]string, 0, len(prevIds))
	for _, item := range prevIds {
		change.PrevIds = append(change.PrevIds, item.GetString())
	}
	return change, nil
}
