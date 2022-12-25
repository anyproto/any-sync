package treestorage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treechangeproto"
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

type Provider interface {
	TreeStorage(id string) (TreeStorage, error)
	CreateTreeStorage(payload TreeStorageCreatePayload) (TreeStorage, error)
}
