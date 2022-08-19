package treecache

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/storage"
	"go.uber.org/zap"
)

const CName = "treecache"

// TODO: add context
type TreeFunc = func(tree interface{}) error

var log = logger.NewNamed("treecache")

type Service interface {
	Do(ctx context.Context, treeId string, f TreeFunc) error
	Add(ctx context.Context, treeId string, header *aclpb.Header, changes []*aclpb.RawChange, f TreeFunc) error
}

type service struct {
	storage storage.Service
	account account.Service
	cache   ocache.OCache
}

func New() app.ComponentRunnable {
	return &service{}
}

func (s *service) Do(ctx context.Context, treeId string, f TreeFunc) error {
	log.
		With(zap.String("treeId", treeId)).
		Debug("requesting tree from cache to perform operation")

	t, err := s.cache.Get(ctx, treeId)
	defer s.cache.Release(treeId)
	if err != nil {
		return err
	}
	return f(t)
}

func (s *service) Add(ctx context.Context, treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange, f TreeFunc) error {
	log.
		With(zap.String("treeId", treeId), zap.Int("len(changes)", len(changes))).
		Debug("adding tree with changes")

	_, err := s.storage.CreateTreeStorage(treeId, header, changes)
	if err != nil {
		return err
	}
	return s.Do(ctx, treeId, f)
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.cache = ocache.New(s.loadTree)
	s.account = a.MustComponent(account.CName).(account.Service)
	s.storage = a.MustComponent(storage.CName).(storage.Service)
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
	t, err := s.storage.TreeStorage(id)
	if err != nil {
		return nil, err
	}
	header, err := t.Header()
	if err != nil {
		return nil, err
	}

	switch header.DocType {
	case aclpb.Header_ACL:
		return tree.BuildACLTreeWithIdentity(t, s.account.Account(), nil)
	case aclpb.Header_DocTree:
		break
	default:
		return nil, fmt.Errorf("incorrect type")
	}
	log.Info("got header", zap.String("header", header.String()))
	var docTree tree.DocTree
	// TODO: it is a question if we need to use ACLTree on the first tree build, because we can think that the tree is already validated
	err = s.Do(ctx, header.AclTreeId, func(obj interface{}) error {
		aclTree := obj.(tree.ACLTree)
		aclTree.RLock()
		defer aclTree.RUnlock()

		docTree, err = tree.BuildDocTreeWithIdentity(t, s.account.Account(), nil, aclTree)
		if err != nil {
			return err
		}
		return nil
	})

	return docTree, err
}
