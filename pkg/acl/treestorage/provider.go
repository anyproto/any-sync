package treestorage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

var ErrUnknownTreeId = errors.New("tree does not exist")

type Provider interface {
	TreeStorage(treeId string) (TreeStorage, error)
	CreateTreeStorage(treeId string, header *aclpb.Header, changes []*aclpb.RawChange) (TreeStorage, error)
}
