package storage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

var ErrUnknownTreeId = errors.New("tree does not exist")

type Provider interface {
	Storage(id string) (Storage, error)
	AddStorage(id string, st Storage) error
	CreateTreeStorage(treeId string, header *aclpb.Header, changes []*aclpb.RawChange) (TreeStorage, error)
}
