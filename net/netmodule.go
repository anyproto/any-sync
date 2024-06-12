//go:generate mockgen -destination mock_net/mock_net.go github.com/anyproto/any-sync/net Service
package net

import (
	"context"
	"net"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/internal/peerservice"
	"github.com/anyproto/any-sync/net/internal/pool"
	"github.com/anyproto/any-sync/net/internal/rpc/server"
	"github.com/anyproto/any-sync/net/internal/streampool"
	"github.com/anyproto/any-sync/net/internal/transport/quic"
	"github.com/anyproto/any-sync/net/peer"
	peer2 "github.com/anyproto/any-sync/net/peer"
	pool2 "github.com/anyproto/any-sync/net/pool"
	streampool2 "github.com/anyproto/any-sync/net/streampool"
)

const CName = "any-sync.net.module"

var log = logger.NewNamed(CName)

type Service interface {
	GetDrpcServer() server.DRPCServer
	NewStreamPool(handler streampool.StreamHandler, config streampool2.StreamConfig) streampool2.StreamPool
	ListenAddrs(ctx context.Context, addrs ...string) (listenAddrs []net.Addr, err error)
	peer2.PeerService
	pool2.Pool
	app.AppModule
}

type netService struct {
	peerService       peerservice.PeerService
	poolService       pool.Service
	streamPoolService streampool.Service
	drpcServer        server.DRPCServer
	quic              quic.Quic
}

func (ns *netService) Inject(a *app.App) {
	ns.poolService = a.MustComponent(pool.CName).(pool.Service)
	ns.peerService = a.MustComponent(peerservice.CName).(peerservice.PeerService)
	ns.streamPoolService = a.MustComponent(streampool.CName).(streampool.Service)
	ns.drpcServer = a.MustComponent(server.CName).(server.DRPCServer)
	ns.quic = a.MustComponent(quic.CName).(quic.Quic)
}

func (ns *netService) PreferQuic(prefer bool) {
	ns.peerService.PreferQuic(prefer)
}

func (ns *netService) SetPeerAddrs(peerId string, addrs []string) {
	ns.peerService.SetPeerAddrs(peerId, addrs)
}

func (ns *netService) Get(ctx context.Context, id string) (peer.Peer, error) {
	return ns.poolService.Get(ctx, id)
}

func (ns *netService) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	return ns.poolService.GetOneOf(ctx, peerIds)
}

func (ns *netService) ListenAddrs(ctx context.Context, addrs ...string) (listenAddrs []net.Addr, err error) {
	return ns.quic.ListenAddrs(ctx, addrs...)
}

func (ns *netService) NewStreamPool(handler streampool.StreamHandler, config streampool2.StreamConfig) streampool2.StreamPool {
	return ns.streamPoolService.NewStreamPool(handler, config)
}

func New() Service {
	return new(netService)
}

func (ns *netService) GetDrpcServer() server.DRPCServer {
	return ns.drpcServer
}

func (ns *netService) Init(a *app.App) (err error) {
	return nil
}

func (ns *netService) Name() (name string) {
	return CName
}
