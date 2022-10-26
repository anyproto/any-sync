package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
)

type TreeStorage interface {
	Id() string
	Root() (*treechangeproto.RawTreeChangeWithId, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error

	AddRawChange(change *treechangeproto.RawTreeChangeWithId) error
	GetRawChange(ctx context.Context, id string) (*treechangeproto.RawTreeChangeWithId, error)
	HasChange(ctx context.Context, id string) (bool, error)
}

type TreeStorageCreatorFunc = func(payload TreeStorageCreatePayload) (TreeStorage, error)
