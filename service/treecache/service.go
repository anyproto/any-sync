package treecache

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"go.uber.org/zap"
)

const CName = "treecache"

// TODO: add context
type ACLTreeFunc = func(tree tree.ACLTree) error
type ChangeBuildFunc = func(builder acltree.ChangeBuilder) error

var log = logger.NewNamed("treecache")

type Service interface {
	Do(ctx context.Context, treeId string, f ACLTreeFunc) error
	Add(ctx context.Context, treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange, f ACLTreeFunc) error
}

type service struct {
	treeProvider treestorage.Provider
	account      account.Service
	cache        ocache.OCache
}

func New() app.ComponentRunnable {
	return &service{}
}

func (s *service) Do(ctx context.Context, treeId string, f ACLTreeFunc) error {
	log.
		With(zap.String("treeId", treeId)).
		Debug("requesting tree from cache to perform operation")

	t, err := s.cache.Get(ctx, treeId)
	defer s.cache.Release(treeId)
	if err != nil {
		return err
	}
	return f(t.(tree.ACLTree))
}

func (s *service) Add(ctx context.Context, treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange, f ACLTreeFunc) error {
	log.
		With(zap.String("treeId", treeId), zap.Int("len(changes)", len(changes))).
		Debug("adding tree with changes")

	_, err := s.treeProvider.CreateTreeStorage(treeId, header, changes)
	if err != nil {
		return err
	}
	return s.Do(ctx, treeId, f)
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
