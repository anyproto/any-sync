package treestorage

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
)

var ErrUnknownTreeId = errors.New("tree does not exist")

type Provider interface {
	TreeStorage(treeId string) (TreeStorage, error)
	CreateTreeStorage(treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange) (TreeStorage, error)
}
