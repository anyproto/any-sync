package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
)

const CName = "common.commonspace"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	CreateSpace(ctx context.Context, cache cache.TreeCache, payload SpaceCreatePayload) (Space, error)
	DeriveSpace(ctx context.Context, cache cache.TreeCache, payload SpaceDerivePayload) (Space, error)
	GetSpace(ctx context.Context, id string) (sp Space, err error)
	app.Component
}

type service struct {
	config               config.Space
	configurationService nodeconf.Service
	storageProvider      storage.SpaceStorageProvider
	cache                cache.TreeCache
	pool                 pool.Pool
}

func (s *service) Init(a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).Space
	s.storageProvider = a.MustComponent(storage.CName).(storage.SpaceStorageProvider)
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.cache = a.MustComponent(cache.CName).(cache.TreeCache)
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateSpace(
	ctx context.Context,
	cache cache.TreeCache,
	payload SpaceCreatePayload) (sp Space, err error) {
	storageCreate, err := storagePayloadForSpaceCreate(payload)
	if err != nil {
		return
	}
	_, err = s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		return
	}

	return s.GetSpace(ctx, storageCreate.Id)
}

func (s *service) DeriveSpace(
	ctx context.Context,
	cache cache.TreeCache,
	payload SpaceDerivePayload) (sp Space, err error) {
	storageCreate, err := storagePayloadForSpaceDerive(payload)
	if err != nil {
		return
	}
	_, err = s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		return
	}

	return s.GetSpace(ctx, storageCreate.Id)
}

func (s *service) GetSpace(ctx context.Context, id string) (Space, error) {
	st, err := s.storageProvider.SpaceStorage(id)
	if err != nil {
		return nil, err
	}
	lastConfiguration := s.configurationService.GetLast()
	confConnector := nodeconf.NewConfConnector(lastConfiguration, s.pool)
	diffService := diffservice.NewDiffService(id, s.config.SyncPeriod, st, confConnector, s.cache, log)
	syncService := syncservice.NewSyncService(id, diffService, s.cache, lastConfiguration, confConnector)
	sp := &space{
		id:          id,
		syncService: syncService,
		diffService: diffService,
		cache:       s.cache,
		storage:     st,
	}
	if err := sp.Init(ctx); err != nil {
		return nil, err
	}
	return sp, nil
}
