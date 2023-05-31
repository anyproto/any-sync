package peerservice

import (
	"context"
	"errors"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
	"sync"
)

const CName = "net.peerservice"

var log = logger.NewNamed(CName)

var (
	ErrAddrsNotFound = errors.New("addrs for peer not found")
)

func New() PeerService {
	return new(peerService)
}

type PeerService interface {
	Dial(ctx context.Context, peerId string) (pr peer.Peer, err error)
	SetPeerAddrs(peerId string, addrs []string)
	transport.Accepter
	app.Component
}

type peerService struct {
	yamux     transport.Transport
	nodeConf  nodeconf.NodeConf
	peerAddrs map[string][]string
	pool      pool.Pool
	server    server.DRPCServer
	mu        sync.RWMutex
}

func (p *peerService) Init(a *app.App) (err error) {
	p.yamux = a.MustComponent(yamux.CName).(transport.Transport)
	p.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	p.pool = a.MustComponent(pool.CName).(pool.Pool)
	p.server = a.MustComponent(server.CName).(server.DRPCServer)
	p.peerAddrs = map[string][]string{}
	return nil
}

func (p *peerService) Name() (name string) {
	return CName
}

func (p *peerService) Dial(ctx context.Context, peerId string) (pr peer.Peer, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	addrs, err := p.getPeerAddrs(peerId)
	if err != nil {
		return
	}

	var mc transport.MultiConn
	log.InfoCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", addrs))
	for _, addr := range addrs {
		mc, err = p.yamux.Dial(ctx, addr)
		if err != nil {
			log.InfoCtx(ctx, "can't connect to host", zap.String("addr", addr), zap.Error(err))
		} else {
			break
		}
	}
	if err != nil {
		return
	}
	return peer.NewPeer(mc, p.server)
}

func (p *peerService) Accept(mc transport.MultiConn) (err error) {
	pr, err := peer.NewPeer(mc, p.server)
	if err != nil {
		return err
	}
	if err = p.pool.AddPeer(context.Background(), pr); err != nil {
		_ = pr.Close()
	}
	return
}

func (p *peerService) SetPeerAddrs(peerId string, addrs []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peerAddrs[peerId] = addrs
}

func (p *peerService) getPeerAddrs(peerId string) ([]string, error) {
	if addrs, ok := p.nodeConf.PeerAddresses(peerId); ok {
		return addrs, nil
	}
	addrs, ok := p.peerAddrs[peerId]
	if !ok || len(addrs) == 0 {
		return nil, ErrAddrsNotFound
	}
	return addrs, nil
}
