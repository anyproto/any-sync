package treecache

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	aclstorage "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/storage"
	"go.uber.org/zap"
)

const CName = "treecache"

type ObjFunc = func(obj interface{}) error

var log = logger.NewNamed("treecache")

type TreePayload struct {
	Header  *aclpb.Header
	Changes []*aclpb.RawChange
}

type ACLListPayload struct {
	Header  *aclpb.Header
	Records []*aclpb.RawRecord
}

type Service interface {
	Do(ctx context.Context, id string, f ObjFunc) error
	Add(ctx context.Context, id string, payload interface{}) error
}

type service struct {
	storage storage.Service
	account account.Service
	cache   ocache.OCache
}

func New() app.ComponentRunnable {
	return &service{}
}

func (s *service) Do(ctx context.Context, treeId string, f ObjFunc) error {
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

func (s *service) Add(ctx context.Context, treeId string, payload interface{}) error {
	switch pl := payload.(type) {
	case TreePayload:
		log.
			With(zap.String("treeId", treeId), zap.Int("len(changes)", len(pl.Changes))).
			Debug("adding Tree with changes")

		_, err := s.storage.CreateTreeStorage(treeId, pl.Header, pl.Changes)
		if err != nil {
			return err
		}
	case ACLListPayload:
		log.
			With(zap.String("treeId", treeId), zap.Int("len(changes)", len(pl.Records))).
			Debug("adding ACLList with records")

		_, err := s.storage.CreateACLListStorage(treeId, pl.Header, pl.Records)
		if err != nil {
			return err
		}

	}
	return nil
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
	t, err := s.storage.Storage(id)
	if err != nil {
		return nil, err
	}
	header, err := t.Header()
	if err != nil {
		return nil, err
	}

	switch header.DocType { // handler
	case aclpb.Header_ACL:
		return list.BuildACLListWithIdentity(s.account.Account(), t.(aclstorage.ListStorage))
	case aclpb.Header_DocTree:
		break
	default:
		return nil, fmt.Errorf("incorrect type")
	}
	log.Info("got header", zap.String("header", header.String()))
	var docTree tree.ObjectTree
	// TODO: it is a question if we need to use ACLList on the first tree build, because we can think that the tree is already validated
	err = s.Do(ctx, header.AclListId, func(obj interface{}) error {
		aclTree := obj.(list.ACLList)
		aclTree.RLock()
		defer aclTree.RUnlock()

		docTree, err = tree.BuildDocTreeWithIdentity(t.(aclstorage.TreeStorage), s.account.Account(), nil, aclTree)
		if err != nil {
			return err
		}
		return nil
	})

	return docTree, err
}
