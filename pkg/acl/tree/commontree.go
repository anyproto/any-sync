package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
)

type CommonTree interface {
	ID() string
	Header() *treepb.TreeHeader
	Heads() []string
	Root() *Change
	Iterate(func(change *Change) bool)
	IterateFrom(string, func(change *Change) bool)
	HasChange(string) bool
	SnapshotPath() []string
	ChangesAfterCommonSnapshot(snapshotPath []string) ([]*aclpb.RawChange, error)
	Storage() treestorage.TreeStorage
	DebugDump() (string, error)
	Close() error
}
