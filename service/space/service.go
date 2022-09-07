package space

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/configuration"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool/handler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/space/spacesync"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"time"
)

const CName = "space"

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type Service interface {
	handler.ReplyHandler
	app.ComponentRunnable
}

type service struct {
	conf        config.Space
	cache       ocache.OCache
	pool        pool.Pool
	confService configuration.Service
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.conf = a.MustComponent(config.CName).(*config.Config).Space
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.confService = a.MustComponent(configuration.CName).(configuration.Service)
	ttlSec := time.Second * time.Duration(s.conf.GCTTL)
	s.cache = ocache.New(s.loadSpace, ocache.WithTTL(ttlSec), ocache.WithGCPeriod(time.Minute))
	s.pool.AddHandler(syncproto.MessageType_MessageTypeSpace, handler.Reply{ReplyHandler: s}.Handle)

	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return
}

func (s *service) loadSpace(ctx context.Context, id string) (value ocache.Object, err error) {
	// TODO: load from database here
	sp := &space{s: s, id: id, conf: s.confService.GetLast()}
	if err = sp.Run(ctx); err != nil {
		return nil, err
	}
	return sp, nil
}

func (s *service) get(ctx context.Context, id string) (Space, error) {
	obj, err := s.cache.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return obj.(Space), nil
}

func (s *service) Handle(ctx context.Context, data []byte) (resp proto.Marshaler, err error) {
	var spaceReq = &spacesync.Space{}
	if err = spaceReq.Unmarshal(data); err != nil {
		return
	}
	if spaceReq.SpaceId != "" {
		sp, e := s.get(ctx, spaceReq.SpaceId)
		if e != nil {
			return nil, e
		}
		return sp.Handle(ctx, spaceReq)
	}
	return nil, fmt.Errorf("unexpected space message")
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.cache.Close()
}
