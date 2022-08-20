package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

var CName = "storage"

type Service interface {
	storage.Provider
}

func New() app.Component {
	return &service{}
}

type service struct {
	storageProvider storage.Provider
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.storageProvider = storage.NewInMemoryTreeStorageProvider()
	return nil
}

func (s *service) Storage(treeId string) (storage.Storage, error) {
	return s.storageProvider.Storage(treeId)
}

func (s *service) CreateTreeStorage(treeId string, header *aclpb.Header, changes []*aclpb.RawChange) (storage.TreeStorage, error) {
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
