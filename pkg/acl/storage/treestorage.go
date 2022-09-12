package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type TreeStorage interface {
	Storage
	Header() (*aclpb.TreeHeader, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error

	AddRawChange(change *aclpb.RawTreeChangeWithId) error
	GetRawChange(ctx context.Context, recordID string) (*aclpb.RawTreeChangeWithId, error)
}

type TreeStorageCreatorFunc = func(payload TreeStorageCreatePayload) (TreeStorage, error)
