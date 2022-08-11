package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
)

var CName = "storage"

type Service interface {
	treestorage.Provider
}

func New() app.Component {
	return &service{}
}

type service struct {
	storageProvider treestorage.Provider
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.storageProvider = treestorage.NewInMemoryTreeStorageProvider()
	return nil
}

func (s *service) TreeStorage(treeId string) (treestorage.TreeStorage, error) {
	return s.storageProvider.TreeStorage(treeId)
}

func (s *service) CreateTreeStorage(treeId string, header *treepb.TreeHeader, changes []*aclpb.RawChange) (treestorage.TreeStorage, error) {
	return s.storageProvider.CreateTreeStorage(treeId, header, changes)
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s service) Close(ctx context.Context) (err error) {
	return nil
}
