package storage

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/etc"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/acllistbuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/node"
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

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.storageProvider = storage.NewInMemoryTreeStorageProvider()
	// importing hardcoded acl list, check that the keys there are correct
	return s.importACLList(a)
}

func (s *service) Storage(treeId string) (storage.Storage, error) {
	return s.storageProvider.Storage(treeId)
}

func (s *service) AddStorage(id string, st storage.Storage) error {
	return s.storageProvider.AddStorage(id, st)
}

func (s *service) CreateTreeStorage(treeId string, header *aclpb.Header, changes []*aclpb.RawChange) (storage.TreeStorage, error) {
	return s.storageProvider.CreateTreeStorage(treeId, header, changes)
}

func (s *service) CreateACLListStorage(id string, header *aclpb.Header, records []*aclpb.RawRecord) (storage.ListStorage, error) {
	return s.storageProvider.CreateACLListStorage(id, header, records)
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

func (s *service) importACLList(a *app.App) (err error) {
	path := fmt.Sprintf("%s/%s", etc.Path(), "acl.yml")
	st, err := acllistbuilder.NewACLListStorageBuilderFromFile(path)
	if err != nil {
		return err
	}

	id, err := st.ID()
	if err != nil {
		return err
	}

	header, err := st.Header()
	if err != nil {
		return err
	}

	// checking that acl list contains all the needed permissions for all our nodes
	err = s.checkActualNodesPermissions(st, a)
	if err != nil {
		return err
	}

	s.importedACLSyncData = ImportedACLSyncData{
		Id:      id,
		Header:  header,
		Records: st.GetRawRecords(),
	}

	log.Infof("imported ACLList with id %s", id)
	return s.storageProvider.AddStorage(id, st)
}

func (s *service) checkActualNodesPermissions(st *acllistbuilder.ACLListStorageBuilder, a *app.App) error {
	nodes := a.MustComponent(node.CName).(node.Service)
	acc := a.MustComponent(account.CName).(account.Service)

	aclList, err := list.BuildACLListWithIdentity(acc.Account(), st)
	if err != nil {
		return err
	}

	state := aclList.ACLState()

	// checking own state
	if state.GetUserStates()[acc.Account().Identity].Permissions != aclpb.ACLChange_Admin {
		return fmt.Errorf("own node with signing key %s should be admin", acc.Account().Identity)
	}

	// checking other nodes' states
	for _, n := range nodes.Nodes() {
		if state.GetUserStates()[n.SigningKeyString].Permissions != aclpb.ACLChange_Admin {
			return fmt.Errorf("other node with signing key %s should be admin", n.SigningKeyString)
		}
	}

	return nil
}
