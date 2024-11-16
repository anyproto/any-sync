package objecttree

import "context"

type StorageChange struct {
	RawChange       []byte
	PrevIds         []string
	Id              string
	SnapshotCounter int
	SnapshotId      string
	OrderId         string
	ChangeSize      int
}

type StorageIterator = func(ctx context.Context, change StorageChange) (shouldContinue bool)

type Storage interface {
	Id() string
	Root() (*StorageChange, error)
	Heads() ([]string, error)
	HasChange(ctx context.Context, id string) (bool, error)
	GetOne(ctx context.Context, id string) (StorageChange, error)
	GetAfterOrder(ctx context.Context, orderId string, iter StorageIterator) error
	AddAll(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error
	Delete() error
}
