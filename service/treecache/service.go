package treecache

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
)

type TreeCache interface {
	Do(ctx context.Context, treeId string, f func(tree acltree.ACLTree)) error
}

type service struct {
	treeProvider treestorage.Provider
	cache        ocache.OCache
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.cache = ocache.New()
}

func (s *service) Name() (name string) {
	//TODO implement me
	panic("implement me")
}

func (s *service) Run(ctx context.Context) (err error) {
	//TODO implement me
	panic("implement me")
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) loadTree(ctx context.Context, id string) (value ocache.Object, err error) {
	tree, err := s.treeProvider.TreeStorage(id)
	if err != nil {
		return nil, err
	}

}
