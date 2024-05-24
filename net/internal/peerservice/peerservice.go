package peerservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/internal/peer"
	"github.com/anyproto/any-sync/net/internal/pool"
	"github.com/anyproto/any-sync/net/internal/rpc/server"
	"github.com/anyproto/any-sync/net/internal/transport"
	"github.com/anyproto/any-sync/net/internal/transport/quic"
	"github.com/anyproto/any-sync/net/internal/transport/yamux"
	peer2 "github.com/anyproto/any-sync/net/peer"
	transport2 "github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "net.peerservice"

var log = logger.NewNamed(CName)

var (
	ErrAddrsNotFound    = errors.New("addrs for peer not found")
	ErrPeerIdMismatched = errors.New("peerId mismatched")
)

func New() PeerService {
	return new(peerService)
}

type PeerService interface {
	Dial(ctx context.Context, peerId string) (pr peer2.Peer, err error)
	peer2.PeerService
	transport.Accepter
	app.Component
}

type peerService struct {
	yamux      transport.Transport
	quic       transport.Transport
	nodeConf   nodeconf.NodeConf
	peerAddrs  map[string][]string
	pool       pool.Pool
	server     server.DRPCServer
	preferQuic bool
	mu         sync.RWMutex
}

func (p *peerService) Init(a *app.App) (err error) {
	p.yamux = a.MustComponent(yamux.CName).(transport.Transport)
	p.quic = a.MustComponent(quic.CName).(transport.Transport)
	p.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	p.pool = a.MustComponent(pool.CName).(pool.Pool)
	p.server = a.MustComponent(server.CName).(server.DRPCServer)
	p.peerAddrs = map[string][]string{}
	p.yamux.SetAccepter(p)
	p.quic.SetAccepter(p)
	return nil
}

var (
	yamuxPreferSchemes = []string{transport2.Yamux, transport2.Quic}
	quicPreferSchemes  = []string{transport2.Quic, transport2.Yamux}
)

func (p *peerService) Name() (name string) {
	return CName
}

func (p *peerService) PreferQuic(prefer bool) {
	p.mu.Lock()
	p.preferQuic = prefer
	p.mu.Unlock()
}

func (p *peerService) Dial(ctx context.Context, peerId string) (pr peer2.Peer, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	addrs, err := p.getPeerAddrs(peerId)
	if err != nil {
		return
	}

	var mc transport.MultiConn
	log.DebugCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", addrs))

	var schemes = yamuxPreferSchemes
	if p.preferQuic {
		schemes = quicPreferSchemes
	}

	err = ErrAddrsNotFound
	for _, sch := range schemes {
		if mc, err = p.dialScheme(ctx, sch, addrs); err == nil {
			break
		}
	}
	if err != nil {
		return
	}
	connPeerId, err := peer2.CtxPeerId(mc.Context())
	if err != nil {
		return nil, err
	}
	if connPeerId != peerId {
		return nil, ErrPeerIdMismatched
	}
	return peer.NewPeer(mc, p.server)
}

func (p *peerService) dialScheme(ctx context.Context, sch string, addrs []string) (mc transport.MultiConn, err error) {
	var tr transport.Transport
	switch sch {
	case transport2.Quic:
		tr = p.quic
	case transport2.Yamux:
		tr = p.yamux
	default:
		return nil, fmt.Errorf("unexpected transport: %v", sch)
	}

	err = ErrAddrsNotFound
	for _, addr := range addrs {
		if scheme(addr) != sch {
			continue
		}
		if mc, err = tr.Dial(ctx, stripScheme(addr)); err == nil {
			return
		} else {
			log.InfoCtx(ctx, "can't connect to host", zap.String("addr", addr), zap.Error(err))
		}
	}
	return
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

func scheme(addr string) string {
	if idx := strings.Index(addr, "://"); idx != -1 {
		return addr[:idx]
	}
	return transport2.Yamux
}

func stripScheme(addr string) string {
	if idx := strings.Index(addr, "://"); idx != -1 {
		return addr[idx+3:]
	}
	return addr
}
