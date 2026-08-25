package peerservice

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/iroh"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/net/transport/webtransport"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
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
	yamux        transport.Transport
	quic         transport.Transport
	webtransport transport.Transport
	iroh         transport.Transport
	nodeConf     nodeconf.NodeConf
	peerAddrs    map[string][]string
	pool         pool.Pool
	server       server.DRPCServer
	preferQuic   bool
	localAddrs   *localAddrDetector
	mu           sync.RWMutex
}

func (p *peerService) Init(a *app.App) (err error) {
	if comp := a.Component(yamux.CName); comp != nil {
		p.yamux = comp.(transport.Transport)
		p.yamux.SetAccepter(p)
	}
	if comp := a.Component(quic.CName); comp != nil {
		p.quic = comp.(transport.Transport)
		p.quic.SetAccepter(p)
	}
	if comp := a.Component(webtransport.CName); comp != nil {
		p.webtransport = comp.(transport.Transport)
		p.webtransport.SetAccepter(p)
	}
	if comp := a.Component(iroh.CName); comp != nil {
		p.iroh = comp.(transport.Transport)
		p.iroh.SetAccepter(p)
	}
	p.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	p.pool = a.MustComponent(pool.CName).(pool.Pool)
	p.server = a.MustComponent(server.CName).(server.DRPCServer)
	p.peerAddrs = map[string][]string{}
	p.localAddrs = newLocalAddrDetector()
	return nil
}

func (p *peerService) preferredSchemes(preferQuic bool) []string {
	var schemes []string
	if preferQuic {
		if p.quic != nil {
			schemes = append(schemes, transport.Quic)
		}
		if p.yamux != nil {
			schemes = append(schemes, transport.Yamux)
		}
	} else {
		if p.yamux != nil {
			schemes = append(schemes, transport.Yamux)
		}
		if p.quic != nil {
			schemes = append(schemes, transport.Quic)
		}
	}
	if p.webtransport != nil {
		schemes = append(schemes, transport.WebTransport)
	}
	// relay dials are the slowest and opt-in (see CtxWithGlobalDial)
	if p.iroh != nil {
		schemes = append(schemes, transport.Iroh)
	}
	return schemes
}

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
	preferQuic := p.preferQuic
	addrs, err := p.getPeerAddrs(peerId)
	p.mu.RUnlock()
	if err != nil {
		return
	}

	// Pass expected peerId in context for transports that need it (e.g. WebTransport)
	ctx = peer.CtxWithExpectedPeerId(ctx, peerId)

	ordered := p.orderAddrs(ctx, addrs, preferQuic)
	log.DebugCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", ordered))

	var mc transport.MultiConn
	err = ErrAddrsNotFound
	for _, addr := range ordered {
		if mc, err = p.dialAddr(ctx, addr); err == nil {
			break
		}
	}
	if err != nil {
		return
	}
	connPeerId, err := peer.CtxPeerId(mc.Context())
	if err != nil {
		_ = mc.Close()
		return nil, err
	}
	if connPeerId != peerId {
		_ = mc.Close()
		return nil, ErrPeerIdMismatched
	}
	pr, err = peer.NewPeer(mc, p.server)
	if err != nil {
		_ = mc.Close()
		return nil, err
	}
	return pr, nil
}

// orderAddrs returns the dial candidates in preference order, dropping addrs
// whose scheme has no registered transport. The scheme order is decided per
// address: local addresses (loopback/private/link-local, hostnames resolved
// with a short deadline) always try yamux first — a dead port answers a TCP
// dial with an RST in one RTT, while a QUIC dial has to wait out the whole
// handshake timeout, because quic-go gets no ICMP feedback on the unconnected
// sockets used for dialing. Non-local addresses follow the global preferQuic
// order. The sort is stable, so the given order is kept within equal
// preference — and with preferQuic unset the order is unchanged, since yamux
// comes first either way. Iroh addresses are tickets, not hosts: they skip
// the local check, always rank last, and are dropped unless the ctx opts in
// to global dials.
func (p *peerService) orderAddrs(ctx context.Context, addrs []string, preferQuic bool) []string {
	type candidate struct {
		addr string
		rank int
	}
	candidates := make([]candidate, 0, len(addrs))
	globalDial := CtxIsGlobalDial(ctx)
	for _, addr := range addrs {
		sch := scheme(addr)
		var schemes []string
		if sch == transport.Iroh {
			if !globalDial {
				continue
			}
			schemes = p.preferredSchemes(preferQuic)
		} else {
			schemes = p.preferredSchemes(preferQuic && !p.localAddrs.isLocal(ctx, stripScheme(addr)))
		}
		rank := slices.Index(schemes, sch)
		if rank == -1 {
			continue
		}
		candidates = append(candidates, candidate{addr: addr, rank: rank})
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		return a.rank - b.rank
	})
	ordered := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ordered = append(ordered, c.addr)
	}
	return ordered
}

func (p *peerService) dialAddr(ctx context.Context, addr string) (mc transport.MultiConn, err error) {
	tr := p.transport(scheme(addr))
	if tr == nil {
		return nil, fmt.Errorf("transport %v not available", scheme(addr))
	}
	if mc, err = tr.Dial(ctx, stripScheme(addr)); err != nil {
		log.InfoCtx(ctx, "can't connect to host", zap.String("addr", addr), zap.Error(err))
	}
	return
}

func (p *peerService) transport(sch string) transport.Transport {
	switch sch {
	case transport.Quic:
		return p.quic
	case transport.Yamux:
		return p.yamux
	case transport.WebTransport:
		return p.webtransport
	case transport.Iroh:
		return p.iroh
	}
	return nil
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
