package treecache

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
)

const CName = "treecache"

type Service interface {
	Do(ctx context.Context, treeId string, f func(tree acltree.ACLTree) error) error
	Add(ctx context.Context, treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange) error
}

type service struct {
	treeProvider treestorage.Provider
	account      account.Service
	cache        ocache.OCache
}

func NewTreeCache() app.ComponentRunnable {
	return &service{}
}

func (s *service) Do(ctx context.Context, treeId string, f func(tree acltree.ACLTree) error) error {
	tree, err := s.cache.Get(ctx, treeId)
	defer s.cache.Release(treeId)
	if err != nil {
		return err
	}
	aclTree := tree.(acltree.ACLTree)
	aclTree.Lock()
	defer aclTree.Unlock()
	return f(tree.(acltree.ACLTree))
}

func (s *service) Add(ctx context.Context, treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange) error {
	_, err := s.treeProvider.CreateTreeStorage(treeId, header, changes)
	if err != nil {
		return err
	}
	// forcing the tree to build
	_, err = s.cache.Get(ctx, treeId)
	return err
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.cache = ocache.New(s.loadTree)
	s.account = a.MustComponent(account.CName).(account.Service)
	s.treeProvider = treestorage.NewInMemoryTreeStorageProvider()
	// TODO: for test we should load some predefined keys
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.cache.Close()
}

func (s *service) loadTree(ctx context.Context, id string) (ocache.Object, error) {
	tree, err := s.treeProvider.TreeStorage(id)
	if err != nil {
		return nil, err
	}
	// TODO: should probably accept nil listeners
	aclTree, err := acltree.BuildACLTree(tree, s.account.Account(), acltree.NoOpListener{})
	return aclTree, err
}
