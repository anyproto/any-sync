package nodespace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/nodespace/nodecache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"time"
)

const CName = "node.nodespace"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	GetSpace(ctx context.Context, id string) (commonspace.Space, error)
	app.ComponentRunnable
}

type service struct {
	conf        config.Space
	spaceCache  ocache.OCache
	commonSpace commonspace.Service
}

func (s *service) Init(a *app.App) (err error) {
	s.conf = a.MustComponent(config.CName).(*config.Config).Space
	s.commonSpace = a.MustComponent(commonspace.CName).(commonspace.Service)
	s.spaceCache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return s.commonSpace.GetSpace(ctx, id, nodecache.NewNodeCache(s.conf.GCTTL))
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

func (s *service) GetSpace(ctx context.Context, id string) (commonspace.Space, error) {
	v, err := s.spaceCache.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return v.(commonspace.Space), nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.spaceCache.Close()
}
