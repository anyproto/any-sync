package netmodule

import (
	"context"
	"net"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peerservice"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/net/transport/quic"
)

const CName = "any-sync.net.module"

var log = logger.NewNamed(CName)

type NetModule interface {
	app.AppModule
	GetDrpcServer() server.DRPCServer
	NewStreamPool(handler streampool.StreamHandler, config streampool.StreamConfig) streampool.StreamPool
	PreferQuic(prefer bool)
	SetPeerAddrs(peerId string, addrs []string)
	Get(ctx context.Context, id string) (peer.Peer, error)
	GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error)
	ListenAddrs(ctx context.Context, addrs ...string) (listenAddrs []net.Addr, err error)
}

type netModule struct {
	peerService       peerservice.PeerService
	poolService       pool.Service
	streamPoolService streampool.Service
	drpcServer        server.DRPCServer
	quic              quic.Quic
}

func (nm *netModule) Inject(a *app.App) {
	nm.poolService = a.MustComponent(pool.CName).(pool.Service)
	nm.peerService = a.MustComponent(peerservice.CName).(peerservice.PeerService)
	nm.streamPoolService = a.MustComponent(streampool.CName).(streampool.Service)
	nm.drpcServer = a.MustComponent(server.CName).(server.DRPCServer)
	nm.quic = a.MustComponent(quic.CName).(quic.Quic)
}

func (nm *netModule) PreferQuic(prefer bool) {
	nm.peerService.PreferQuic(prefer)
}

func (nm *netModule) SetPeerAddrs(peerId string, addrs []string) {
	nm.peerService.SetPeerAddrs(peerId, addrs)
}

func (nm *netModule) Get(ctx context.Context, id string) (peer.Peer, error) {
	return nm.poolService.Get(ctx, id)
}

func (nm *netModule) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	return nm.poolService.GetOneOf(ctx, peerIds)
}

func (nm *netModule) ListenAddrs(ctx context.Context, addrs ...string) (listenAddrs []net.Addr, err error) {
	return nm.quic.ListenAddrs(ctx, addrs...)
}

func (nm *netModule) NewStreamPool(handler streampool.StreamHandler, config streampool.StreamConfig) streampool.StreamPool {
	return nm.streamPoolService.NewStreamPool(handler, config)
}

func New() NetModule {
	return new(netModule)
}

func (nm *netModule) GetDrpcServer() server.DRPCServer {
	return nm.drpcServer
}

func (nm *netModule) Init(a *app.App) (err error) {
	return nil
}

func (nm *netModule) Name() (name string) {
	return CName
}
