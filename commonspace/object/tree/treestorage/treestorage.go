//go:generate mockgen -destination mock_treestorage/mock_treestorage.go github.com/anyproto/any-sync/commonspace/object/tree/treestorage TreeStorage
package treestorage

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
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

type Exporter interface {
	TreeStorage(root *treechangeproto.RawTreeChangeWithId) (TreeStorage, error)
}

type TreeStorageCreatorFunc = func(payload TreeStorageCreatePayload) (TreeStorage, error)

type TreeStorage interface {
	Id() string
	Root() (*treechangeproto.RawTreeChangeWithId, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error
	AddRawChange(change *treechangeproto.RawTreeChangeWithId) error
	AddRawChangesSetHeads(changes []*treechangeproto.RawTreeChangeWithId, heads []string) error

	GetRawChange(ctx context.Context, id string) (*treechangeproto.RawTreeChangeWithId, error)
	GetAppendRawChange(ctx context.Context, buf []byte, id string) (*treechangeproto.RawTreeChangeWithId, error)
	HasChange(ctx context.Context, id string) (bool, error)
	Delete() error
}
