package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
)

const CName = "common.commonspace"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	CreateSpace(ctx context.Context, id string, deps SpaceDeps) (sp Space, err error)
	app.Component
}

type service struct {
	config               config.Space
	configurationService nodeconf.Service
}

type SpaceDeps struct {
	Cache   cache.TreeCache
	Storage storage.Storage
}

func (s *service) Init(a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).Space
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateSpace(ctx context.Context, id string, deps SpaceDeps) (Space, error) {
	lastConfiguration := s.configurationService.GetLast()
	diffService := diffservice.NewDiffService(id, s.config.SyncPeriod, deps.Storage, lastConfiguration, deps.Cache, log)
	syncService := syncservice.NewSyncService(id, diffService, deps.Cache, lastConfiguration)
	sp := &space{
		id:          id,
		conf:        s.config,
		syncService: syncService,
		diffService: diffService,
		cache:       deps.Cache,
		storage:     deps.Storage,
	}
	if err := sp.Init(ctx); err != nil {
		return nil, err
	}
	return sp, nil
}
