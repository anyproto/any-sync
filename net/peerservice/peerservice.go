package peerservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/net/transport/yamux"
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
	Dial(ctx context.Context, peerId string) (pr peer.Peer, err error)
	SetPeerAddrs(peerId string, addrs []string)
	PreferQuic(prefer bool)
	transport.Accepter
	app.Component
}

type peerService struct {
	yamux         transport.Transport
	quic          transport.Transport
	nodeConf      nodeconf.NodeConf
	peerAddrs     map[string][]string
	ignoreAddrs   *sync.Map
	ignoreTimeout time.Duration
	pool          pool.Pool
	server        server.DRPCServer
	preferQuic    bool
	mu            sync.RWMutex
}

func (p *peerService) Init(a *app.App) (err error) {
	p.yamux = a.MustComponent(yamux.CName).(transport.Transport)
	p.quic = a.MustComponent(quic.CName).(transport.Transport)
	p.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	p.pool = a.MustComponent(pool.CName).(pool.Pool)
	p.server = a.MustComponent(server.CName).(server.DRPCServer)
	p.peerAddrs = map[string][]string{}
	p.ignoreAddrs = &sync.Map{}
	p.ignoreTimeout = time.Minute * 3
	p.yamux.SetAccepter(p)
	p.quic.SetAccepter(p)
	return nil
}

var (
	yamuxPreferSchemes = []string{transport.Yamux, transport.Quic}
	quicPreferSchemes  = []string{transport.Quic, transport.Yamux}
)

func (p *peerService) Name() (name string) {
	return CName
}

func (p *peerService) PreferQuic(prefer bool) {
	p.mu.Lock()
	p.preferQuic = prefer
	p.mu.Unlock()
}

func (p *peerService) Dial(ctx context.Context, peerId string) (pr peer.Peer, err error) {
	p.mu.RLock()
	var schemes = yamuxPreferSchemes
	if p.preferQuic {
		schemes = quicPreferSchemes
	}

	addrs, err := p.getPeerAddrs(peerId)
	if err != nil {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	var mc transport.MultiConn
	log.DebugCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", addrs))

	err = ErrAddrsNotFound
	for _, sch := range schemes {
		if mc, err = p.dialScheme(ctx, sch, addrs); err == nil {
			break
		}
	}
	if err != nil {
		return
	}
	connPeerId, err := peer.CtxPeerId(mc.Context())
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
	case transport.Quic:
		tr = p.quic
	case transport.Yamux:
		tr = p.yamux
	default:
		return nil, fmt.Errorf("unexpected transport: %v", sch)
	}

	err = ErrAddrsNotFound
	now := time.Now()
	for _, addr := range addrs {
		if scheme(addr) != sch {
			continue
		}
		if tm, ok := p.ignoreAddrs.Load(addr); ok && tm.(time.Time).After(now) {
			continue
		}
		if mc, err = tr.Dial(ctx, stripScheme(addr)); err == nil {
			p.ignoreAddrs.Delete(addr)
			return
		} else {
			log.InfoCtx(ctx, "can't connect to host", zap.String("addr", addr), zap.Error(err))
			p.ignoreAddrs.Store(addr, time.Now().Add(p.ignoreTimeout))
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
	return transport.Yamux
}

func stripScheme(addr string) string {
	if idx := strings.Index(addr, "://"); idx != -1 {
		return addr[idx+3:]
	}
	return addr
}
