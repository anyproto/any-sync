package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	config2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
)

const CName = "common.commonspace"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	DeriveSpace(ctx context.Context, payload SpaceDerivePayload) (string, error)
	CreateSpace(ctx context.Context, payload SpaceCreatePayload) (string, error)
	GetSpace(ctx context.Context, id string) (sp Space, err error)
	app.Component
}

type service struct {
	config               config2.Space
	account              account.Service
	configurationService nodeconf.Service
	storageProvider      storage.SpaceStorageProvider
	cache                treegetter.TreeGetter
	pool                 pool.Pool
}

func (s *service) Init(a *app.App) (err error) {
	s.config = a.MustComponent(config2.CName).(*config2.Config).Space
	s.account = a.MustComponent(account.CName).(account.Service)
	s.storageProvider = a.MustComponent(storage.CName).(storage.SpaceStorageProvider)
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.cache = a.MustComponent(treegetter.CName).(treegetter.TreeGetter)
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateSpace(
	ctx context.Context,
	payload SpaceCreatePayload) (id string, err error) {
	storageCreate, err := storagePayloadForSpaceCreate(payload)
	if err != nil {
		return
	}
	store, err := s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		return
	}

	return store.ID()
}

func (s *service) DeriveSpace(
	ctx context.Context,
	payload SpaceDerivePayload) (id string, err error) {
	storageCreate, err := storagePayloadForSpaceDerive(payload)
	if err != nil {
		return
	}
	store, err := s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		return
	}

	return store.ID()
}

func (s *service) GetSpace(ctx context.Context, id string) (Space, error) {
	st, err := s.storageProvider.SpaceStorage(id)
	if err != nil {
		return nil, err
	}

	lastConfiguration := s.configurationService.GetLast()
	confConnector := nodeconf.NewConfConnector(lastConfiguration, s.pool)
	diffService := diffservice.NewDiffService(id, s.config.SyncPeriod, st, confConnector, s.cache, log)
	syncService := syncservice.NewSyncService(id, confConnector)
	sp := &space{
		id:            id,
		syncService:   syncService,
		diffService:   diffService,
		cache:         s.cache,
		account:       s.account,
		configuration: lastConfiguration,
		storage:       st,
	}
	if err := sp.Init(ctx); err != nil {
		return nil, err
	}
	return sp, nil
}
