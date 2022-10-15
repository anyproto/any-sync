package storage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
)

var ErrUnknownTreeId = errors.New("tree does not exist")
var ErrTreeExists = errors.New("tree already exists")
var ErrUnkownChange = errors.New("change doesn't exist")

type TreeStorageCreatePayload struct {
	TreeId        string
	RootRawChange *treechangeproto.RawTreeChangeWithId
	Changes       []*treechangeproto.RawTreeChangeWithId
	Heads         []string
}

type Provider interface {
	TreeStorage(id string) (TreeStorage, error)
	CreateTreeStorage(payload TreeStorageCreatePayload) (TreeStorage, error)
}
