package objecttree

import (
	"context"
	"fmt"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
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

	return st, nil
}

func (l *lazyStorage) createStorage(ctx context.Context) error {
	store, err := CreateStorageTx(ctx, l.root.RawTreeChangeWithId(), l.headStorage, l.store)
	if err != nil {
		return fmt.Errorf("create storage: %w", err)
	}
	l.storage = store
	return nil
}

func (l *lazyStorage) Id() string {
	return l.id
}

func (l *lazyStorage) Root(ctx context.Context) (StorageChange, error) {
	return l.root, nil
}

func (l *lazyStorage) Heads(ctx context.Context) ([]string, error) {
	if l.storage != nil {
		return l.storage.Heads(ctx)
	}
	return l.heads, nil
}

func (l *lazyStorage) CommonSnapshot(ctx context.Context) (string, error) {
	if l.storage != nil {
		return l.storage.CommonSnapshot(ctx)
	}
	return l.id, nil
}

func (l *lazyStorage) Has(ctx context.Context, id string) (bool, error) {
	if l.storage != nil {
		return l.storage.Has(ctx, id)
	}
	return id == l.id, nil
}

func (l *lazyStorage) Get(ctx context.Context, id string) (StorageChange, error) {
	if l.storage != nil {
		return l.storage.Get(ctx, id)
	}
	return l.root, nil
}

func (l *lazyStorage) GetAfterOrder(ctx context.Context, orderId string, iter StorageIterator) error {
	if l.storage != nil {
		return l.storage.GetAfterOrder(ctx, orderId, iter)
	}
	if orderId <= l.root.OrderId {
		_, err := iter(ctx, l.root)
		return err
	}
	return nil
}

func (l *lazyStorage) createStorageAndDoInTx(ctx context.Context, proc func(ctx context.Context) error) error {
	tx, err := l.store.WriteTx(ctx)
	if err != nil {
		return fmt.Errorf("write tx: %w", err)
	}
	defer tx.Rollback()

	err = l.createStorage(tx.Context())
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}

	err = proc(tx.Context())
	if err != nil {
		return fmt.Errorf("add all: %w", err)
	}
	return tx.Commit()
}

func (l *lazyStorage) AddAll(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error {
	if l.storage == nil {
		return l.createStorageAndDoInTx(ctx, func(ctx context.Context) error {
			return l.storage.AddAll(ctx, changes, heads, commonSnapshot)
		})
	}
	return l.storage.AddAll(ctx, changes, heads, commonSnapshot)
}

func (l *lazyStorage) AddAllNoError(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error {
	if l.storage == nil {
		return l.createStorageAndDoInTx(ctx, func(ctx context.Context) error {
			return l.storage.AddAllNoError(ctx, changes, heads, commonSnapshot)
		})
	}
	return l.storage.AddAllNoError(ctx, changes, heads, commonSnapshot)
}

func (l *lazyStorage) Delete(ctx context.Context) error {
	if l.storage == nil {
		return l.createStorageAndDoInTx(ctx, func(ctx context.Context) error {
			return l.storage.Delete(ctx)
		})
	}
	return l.storage.Delete(ctx)
}

func (l *lazyStorage) Close() error {
	if l.storage != nil {
		return l.storage.Close()
	}
	return nil
}
