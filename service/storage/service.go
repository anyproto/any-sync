package storage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

var CName = "storage"

var log = logger.NewNamed("storage").Sugar()

type ImportedACLSyncData struct {
	Id      string
	Header  *aclpb.Header
	Records []*aclpb.RawACLRecord
}

type Service interface {
	storage.Provider
	ImportedACLSyncData() ImportedACLSyncData
}

func New() app.Component {
	return &service{}
}

type service struct {
	storageProvider     storage.Provider
	importedACLSyncData ImportedACLSyncData
}

func (s *service) Init(a *app.App) (err error) {
	s.storageProvider = storage.NewInMemoryTreeStorageProvider()
	// importing hardcoded acl list, check that the keys there are correct
	return nil
}

func (s *service) Storage(treeId string) (storage.Storage, error) {
	return s.storageProvider.Storage(treeId)
}

func (s *service) AddStorage(id string, st storage.Storage) error {
	return s.storageProvider.AddStorage(id, st)
}

func (s *service) CreateTreeStorage(payload storage.TreeStorageCreatePayload) (storage.TreeStorage, error) {
	return s.storageProvider.CreateTreeStorage(payload)
}

func (s *service) CreateACLListStorage(payload storage.ACLListStorageCreatePayload) (storage.ListStorage, error) {
	return s.storageProvider.CreateACLListStorage(payload)
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) ImportedACLSyncData() ImportedACLSyncData {
	return s.importedACLSyncData
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s service) Close(ctx context.Context) (err error) {
	return nil
}
