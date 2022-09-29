package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
)

type TreeStorage interface {
	Storage
	Root() (*treechangeproto.RawTreeChangeWithId, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error

	AddRawChange(change *treechangeproto.RawTreeChangeWithId) error
	GetRawChange(ctx context.Context, id string) (*treechangeproto.RawTreeChangeWithId, error)
}

type TreeStorageCreatorFunc = func(payload TreeStorageCreatePayload) (TreeStorage, error)
