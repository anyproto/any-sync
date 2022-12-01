package client

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"net"
	"storj.io/drpc/drpcconn"
	"time"
)

type Service interface {
	Get(ctx context.Context, ip string) (apiproto.DRPCClientApiClient, error)
	app.ComponentRunnable
}

const CName = "debug.client"

var log = logger.NewNamed(CName)

type service struct {
	cache ocache.OCache
}

func New() Service {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	s.cache = ocache.New(
		func(ctx context.Context, ip string) (value ocache.Object, err error) {
			conn, err := net.Dial("tcp", ip)
			if err != nil {
				return
			}
			value = drpcconn.New(conn)
			return
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Minute*5),
	)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s *service) Get(ctx context.Context, ip string) (apiproto.DRPCClientApiClient, error) {
	v, err := s.cache.Get(ctx, ip)
	if err != nil {
		return nil, err
	}
	conn := v.(*drpcconn.Conn)
	select {
	case <-conn.Closed():
	default:
		return apiproto.NewDRPCClientApiClient(conn), nil
	}
	s.cache.Remove(ip)
	return s.Get(ctx, ip)
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.cache.Close()
}
