package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
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
	CreateSpace(ctx context.Context, id string) (sp Space, err error)
	app.Component
}

type service struct {
	config               config.Space
	configurationService nodeconf.Service
	storage              storage.Storage
	cache                cache.TreeCache
}

func (s *service) Init(a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).Space
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.storage = a.MustComponent(storage.CName).(storage.Storage)
	s.cache = a.MustComponent(cache.CName).(cache.TreeCache)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateSpace(ctx context.Context, id string) (Space, error) {
	syncService := syncservice.NewSyncService(id, nil, s.configurationService.GetLast())
	sp := &space{
		id:          id,
		nconf:       s.configurationService.GetLast(),
		conf:        s.config,
		syncService: syncService,
		cache:       s.cache,
		storage:     s.storage,
	}
	if err := sp.Init(ctx); err != nil {
		return nil, err
	}
	return sp, nil
}
