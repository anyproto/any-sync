package treecache

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	aclstorage "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/storage"
	"go.uber.org/zap"
)

const CName = "treecache"

type ObjFunc = func(obj interface{}) error

var log = logger.NewNamed("treecache")

type Service interface {
	Do(ctx context.Context, id string, f ObjFunc) error
	Add(ctx context.Context, id string, payload any) error
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

func (s *service) Add(ctx context.Context, treeId string, payload any) error {
	switch pl := payload.(type) {
	case aclstorage.TreeStorageCreatePayload:
		log.
			With(zap.String("treeId", treeId), zap.Int("len(changes)", len(pl.Changes))).
			Debug("adding Tree with changes")

		_, err := s.storage.CreateTreeStorage(payload.(aclstorage.TreeStorageCreatePayload))
		if err != nil {
			return err
		}
	case aclstorage.ACLListStorageCreatePayload:
		log.
			With(zap.String("treeId", treeId), zap.Int("len(changes)", len(pl.Records))).
			Debug("adding ACLList with records")

		_, err := s.storage.CreateACLListStorage(payload.(aclstorage.ACLListStorageCreatePayload))
		if err != nil {
			return err
		}

	}
	return nil
}

func (s *service) Init(a *app.App) (err error) {
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
	var objTree tree.ObjectTree
	err = s.Do(ctx, header.AclListId, func(obj interface{}) error {
		aclList := obj.(list.ACLList)
		objTree, err = tree.BuildObjectTree(t.(aclstorage.TreeStorage), nil, aclList)
		if err != nil {
			return err
		}
		return nil
	})

	return objTree, err
}
