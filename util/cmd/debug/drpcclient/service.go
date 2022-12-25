package drpcclient

import (
	"context"
	clientproto "github.com/anytypeio/go-anytype-infrastructure-experiments/client/debug/clientdebugrpc/clientdebugrpcproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/ocache"
	nodeproto "github.com/anytypeio/go-anytype-infrastructure-experiments/node/debug/nodedebugrpc/nodedebugrpcproto"
	"net"
	"storj.io/drpc/drpcconn"
	"time"
)

type Service interface {
	GetClient(ctx context.Context, ip string) (clientproto.DRPCClientApiClient, error)
	GetNode(ctx context.Context, ip string) (nodeproto.DRPCNodeApiClient, error)
	app.ComponentRunnable
}

const CName = "debug.drpcclient"

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

func (s *service) GetClient(ctx context.Context, ip string) (clientproto.DRPCClientApiClient, error) {
	v, err := s.cache.Get(ctx, ip)
	if err != nil {
		return nil, err
	}
	conn := v.(*drpcconn.Conn)
	select {
	case <-conn.Closed():
	default:
		return clientproto.NewDRPCClientApiClient(conn), nil
	}
	s.cache.Remove(ip)
	return s.GetClient(ctx, ip)
}

func (s *service) GetNode(ctx context.Context, ip string) (nodeproto.DRPCNodeApiClient, error) {
	v, err := s.cache.Get(ctx, ip)
	if err != nil {
		return nil, err
	}
	conn := v.(*drpcconn.Conn)
	select {
	case <-conn.Closed():
	default:
		return nodeproto.NewDRPCNodeApiClient(conn), nil
	}
	s.cache.Remove(ip)
	return s.GetNode(ctx, ip)
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.cache.Close()
}
