package spacetree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
)

type SpaceTree interface {
	cache.TreeContainer
	ID() string
	GetObjectIds() []string
	Sync()
}

type spaceTree struct{}

func (s *spaceTree) Tree() tree.ObjectTree {
	//TODO implement me
	panic("implement me")
}

func (s *spaceTree) ID() string {
	//TODO implement me
	panic("implement me")
}

func (s *spaceTree) GetObjectIds() []string {
	//TODO implement me
	panic("implement me")
}

func (s *spaceTree) Sync() {
	//TODO implement me
	panic("implement me")
}

func NewSpaceTree(id string) (SpaceTree, error) {
	return &spaceTree{}, nil
}
