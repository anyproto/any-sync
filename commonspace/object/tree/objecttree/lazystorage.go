package objecttree

import (
	"context"
	"errors"
	"fmt"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
)

type lazyStorage struct {
	id string

	store       anystore.DB
	headStorage headstorage.HeadStorage

	storage Storage

	heads            []string
	root             StorageChange
	unmarshalledRoot *Change
}

func CreateLazyStorage(ctx context.Context, root *treechangeproto.RawTreeChangeWithId, headStorage headstorage.HeadStorage, store anystore.DB) (Storage, error) {

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

	st := &lazyStorage{
		id:          root.Id,
		store:       store,
		headStorage: headStorage,
		root:        stChange,
		heads:       []string{root.Id},
	}
	ok, err := st.isRootChangeStored(ctx)
	if err != nil {
		return nil, fmt.Errorf("check if storage exists: %w", err)
	}
	if ok {
		return nil, treestorage.ErrTreeExists
	}
	return st, nil
}

func (s *lazyStorage) isRootChangeStored(ctx context.Context) (bool, error) {
	changesColl, err := s.store.OpenCollection(ctx, CollName)
	if errors.Is(err, anystore.ErrCollectionNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = changesColl.FindId(ctx, s.root.Id)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, anystore.ErrDocNotFound) {
		return false, nil
	}
	return false, err
}

func (s *lazyStorage) createStorage(ctx context.Context) error {
	store, err := CreateStorageTx(ctx, s.root.RawTreeChangeWithId(), s.headStorage, s.store)
	if err != nil {
		return fmt.Errorf("create storage: %w", err)
	}
	s.storage = store
	return nil
}

func (s *lazyStorage) Id() string {
	return s.id
}

func (s *lazyStorage) Root(ctx context.Context) (StorageChange, error) {
	return s.root, nil
}

func (s *lazyStorage) Heads(ctx context.Context) ([]string, error) {
	if s.storage != nil {
		return s.storage.Heads(ctx)
	}
	return s.heads, nil
}

func (s *lazyStorage) CommonSnapshot(ctx context.Context) (string, error) {
	if s.storage != nil {
		return s.storage.CommonSnapshot(ctx)
	}
	return s.id, nil
}

func (s *lazyStorage) Has(ctx context.Context, id string) (bool, error) {
	if s.storage != nil {
		return s.storage.Has(ctx, id)
	}
	return id == s.id, nil
}

func (s *lazyStorage) Get(ctx context.Context, id string) (StorageChange, error) {
	if s.storage != nil {
		return s.storage.Get(ctx, id)
	}
	return s.root, nil
}

func (s *lazyStorage) GetAfterOrder(ctx context.Context, orderId string, iter StorageIterator) error {
	if s.storage != nil {
		return s.storage.GetAfterOrder(ctx, orderId, iter)
	}
	if orderId <= s.root.OrderId {
		_, err := iter(ctx, s.root)
		return err
	}
	return nil
}

func (s *lazyStorage) createStorageAndDoInTx(ctx context.Context, proc func(ctx context.Context) error) error {
	tx, err := s.store.WriteTx(ctx)
	if err != nil {
		return fmt.Errorf("write tx: %w", err)
	}
	defer tx.Rollback()

	err = s.createStorage(tx.Context())
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}

	err = proc(tx.Context())
	if err != nil {
		return fmt.Errorf("add all: %w", err)
	}
	return tx.Commit()
}

func (s *lazyStorage) AddAll(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error {
	if s.storage == nil {
		return s.createStorageAndDoInTx(ctx, func(ctx context.Context) error {
			return s.storage.AddAll(ctx, changes, heads, commonSnapshot)
		})
	}
	return s.storage.AddAll(ctx, changes, heads, commonSnapshot)
}

func (s *lazyStorage) AddAllNoError(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error {
	if s.storage == nil {
		return s.createStorageAndDoInTx(ctx, func(ctx context.Context) error {
			return s.storage.AddAllNoError(ctx, changes, heads, commonSnapshot)
		})
	}
	return s.storage.AddAllNoError(ctx, changes, heads, commonSnapshot)
}

func (s *lazyStorage) Delete(ctx context.Context) error {
	if s.storage == nil {
		return s.createStorageAndDoInTx(ctx, func(ctx context.Context) error {
			return s.storage.Delete(ctx)
		})
	}
	return s.storage.Delete(ctx)
}

func (s *lazyStorage) Close() error {
	if s.storage != nil {
		return s.storage.Close()
	}
	return nil
}
