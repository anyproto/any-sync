package treestorage

import (
	"context"
	"errors"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
)

var (
	ErrUnknownTreeId = errors.New("tree does not exist")
	ErrTreeExists    = errors.New("tree already exists")
	ErrUnknownChange = errors.New("change doesn't exist")
)

type TreeStorageCreatePayload struct {
	RootRawChange *treechangeproto.RawTreeChangeWithId
	Changes       []*treechangeproto.RawTreeChangeWithId
	Heads         []string
}

type TreeStorageCreatorFunc = func(payload TreeStorageCreatePayload) (TreeStorage, error)

type TreeStorage interface {
	Id() string
	Root() (*treechangeproto.RawTreeChangeWithId, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error

	AddRawChange(change *treechangeproto.RawTreeChangeWithId) error
	GetRawChange(ctx context.Context, id string) (*treechangeproto.RawTreeChangeWithId, error)
	HasChange(ctx context.Context, id string) (bool, error)
	Delete() error
}
