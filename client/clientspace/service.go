package clientspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/util"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	config2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"time"
)

const CName = "client.clientspace"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	GetSpace(ctx context.Context, id string) (util.ReleaseContainer[commonspace.Space], error)
	CreateSpace(ctx context.Context, payload commonspace.SpaceCreatePayload) (util.ReleaseContainer[commonspace.Space], error)
	DeriveSpace(ctx context.Context, payload commonspace.SpaceDerivePayload) (util.ReleaseContainer[commonspace.Space], error)
	app.ComponentRunnable
}

type service struct {
	conf                 config2.Space
	spaceCache           ocache.OCache
	commonSpace          commonspace.Service
	spaceStorageProvider storage.SpaceStorageProvider
}

func (s *service) Init(a *app.App) (err error) {
	s.conf = a.MustComponent(config2.CName).(*config2.Config).Space
	s.commonSpace = a.MustComponent(commonspace.CName).(commonspace.Service)
	s.spaceStorageProvider = a.MustComponent(storage.CName).(storage.SpaceStorageProvider)
	s.spaceCache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return s.commonSpace.GetSpace(ctx, id)
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Duration(s.conf.GCTTL)*time.Second),
		ocache.WithRefCounter(false),
	)
	return spacesyncproto.DRPCRegisterSpace(a.MustComponent(server.CName).(server.DRPCServer), &rpcHandler{s})
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	go func() {
		time.Sleep(time.Second * 5)
		_, _ = s.GetSpace(ctx, "testDSpace")
	}()
	return
}

func (s *service) CreateSpace(ctx context.Context, payload commonspace.SpaceCreatePayload) (container util.ReleaseContainer[commonspace.Space], err error) {
	id, err := s.commonSpace.CreateSpace(ctx, payload)
	if err != nil {
		return
	}

	obj, err := s.spaceCache.Get(ctx, id)
	if err != nil {
		return
	}
	return util.ReleaseContainer[commonspace.Space]{
		Object: obj.(commonspace.Space),
		Release: func() {
			s.spaceCache.Release(id)
		},
	}, nil
}

func (s *service) DeriveSpace(ctx context.Context, payload commonspace.SpaceDerivePayload) (container util.ReleaseContainer[commonspace.Space], err error) {
	id, err := s.commonSpace.DeriveSpace(ctx, payload)
	if err != nil {
		return
	}

	obj, err := s.spaceCache.Get(ctx, id)
	if err != nil {
		return
	}
	return util.ReleaseContainer[commonspace.Space]{
		Object: obj.(commonspace.Space),
		Release: func() {
			s.spaceCache.Release(id)
		},
	}, nil
}

func (s *service) GetSpace(ctx context.Context, id string) (container util.ReleaseContainer[commonspace.Space], err error) {
	v, err := s.spaceCache.Get(ctx, id)
	if err != nil {
		return
	}
	return util.ReleaseContainer[commonspace.Space]{
		Object: v.(commonspace.Space),
		Release: func() {
			s.spaceCache.Release(id)
		},
	}, nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.spaceCache.Close()
}
