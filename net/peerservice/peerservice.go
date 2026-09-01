package peerservice

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peerobserver"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/transport"
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
	// EnableQuicDemotion turns on QUIC degradation tracking: peers whose QUIC
	// connections keep dying under DPI-style degradation get demoted to
	// yamux-first dialing. Opt-in so server nodes keep their dial behavior
	// unchanged; clients call this once at startup.
	EnableQuicDemotion()
	// TransportPenalties returns a copy of the QUIC penalty state, for
	// persistence across restarts.
	TransportPenalties() PenaltySnapshot
	// SeedTransportPenalties replaces the QUIC penalty state with a
	// previously stored snapshot.
	SeedTransportPenalties(snap PenaltySnapshot)
	// ResetTransportPenalties drops all QUIC penalty state (e.g. after a
	// network change: a new network deserves a clean verdict).
	ResetTransportPenalties()
	// SetPenaltyObserver registers a callback fired after every penalty state
	// mutation, so a client can persist the snapshot. Must not block.
	SetPenaltyObserver(observer func())
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
	penalties    *transportPenalties
	// observer is bound once at Init (peerobserver.CName) and never changes
	// afterwards, so it is read without locking; its zero value is a no-op
	observer peerobserver.Notifier
	mu       sync.RWMutex
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
	if comp := a.Component(transport.IrohCName); comp != nil {
		p.iroh = comp.(transport.Transport)
		p.iroh.SetAccepter(p)
	}
	p.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	p.pool = a.MustComponent(pool.CName).(pool.Pool)
	p.server = a.MustComponent(server.CName).(server.DRPCServer)
	p.peerAddrs = map[string][]string{}
	p.localAddrs = newLocalAddrDetector()
	p.penalties = newTransportPenalties(time.Now, func(peerId string) bool {
		return len(p.nodeConf.NodeTypes(peerId)) > 0
	})
	p.observer = peerobserver.FromApp(a)
	return nil
}

func (p *peerService) EnableQuicDemotion() {
	p.penalties.enable()
	if setter, ok := p.quic.(transport.ConnObserverSetter); ok {
		setter.SetConnObserver(p.onConnClosed)
	}
}

// onConnClosed receives close events of dialed QUIC connections and feeds the
// penalty state: degraded deaths accumulate strikes toward demoting the peer
// to yamux-first, a healthy connection clears them.
func (p *peerService) onConnClosed(ev transport.ConnCloseEvent) {
	if ev.PeerId == "" {
		return
	}
	switch ev.Kind {
	case transport.ConnCloseDegraded:
		demoted := p.penalties.registerDegraded(ev.PeerId)
		log.Info("quic conn died degraded",
			zap.String("peerId", ev.PeerId),
			zap.Duration("lifetime", ev.Lifetime),
			zap.Int64("bytesRead", ev.BytesRead),
			zap.Int64("bytesWritten", ev.BytesWritten),
			zap.Bool("demoted", demoted),
			zap.Error(ev.Cause))
	case transport.ConnCloseHealthy:
		p.penalties.registerHealthy(ev.PeerId)
	}
}

func (p *peerService) TransportPenalties() PenaltySnapshot {
	return p.penalties.snapshot()
}

func (p *peerService) SeedTransportPenalties(snap PenaltySnapshot) {
	p.penalties.seed(snap)
}

func (p *peerService) ResetTransportPenalties() {
	log.Info("transport penalties reset")
	p.penalties.reset()
}

func (p *peerService) SetPenaltyObserver(observer func()) {
	p.penalties.setObserver(observer)
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
	dialStarted := time.Now()
	p.mu.RLock()
	preferQuic := p.preferQuic
	addrs, err := p.getPeerAddrs(peerId)
	p.mu.RUnlock()
	if err != nil {
		p.notifyDialStarted(peerId, 0)
		p.notifyDialFailed(peerId, err, dialStarted)
		return
	}

	// Pass expected peerId in context for transports that need it (e.g. WebTransport)
	ctx = peer.CtxWithExpectedPeerId(ctx, peerId)

	if preferQuic && p.penalties.quicDemoted(peerId) {
		preferQuic = false
	}
	ordered := p.orderAddrs(ctx, addrs, preferQuic)
	log.DebugCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", logAddrs(ordered)))
	p.notifyDialStarted(peerId, len(ordered))

	var (
		mc           transport.MultiConn
		quicDegraded bool
		connAddr     string
		addrErrs     []error
	)
	err = ErrAddrsNotFound
	for _, addr := range ordered {
		if mc, err = p.dialAddr(ctx, addr); err == nil {
			connAddr = addr
			break
		}
		addrErrs = append(addrErrs, err)
		if scheme(addr) == transport.Quic && quic.IsHandshakeTimeout(err) {
			quicDegraded = true
		}
	}
	if quicDegraded {
		// one strike per Dial, however many quic addrs timed out: UDP toward
		// this peer looks blocked while other schemes may still work
		if p.penalties.registerDegraded(peerId) {
			log.Info("quic demoted after handshake timeouts", zap.String("peerId", peerId))
		}
	}
	if err != nil {
		// Dial keeps returning the last error; the observer gets every
		// per-address error joined, since the first ones are often the
		// informative ones (a refused TCP port says more than a QUIC
		// timeout). Always joined, so consumers see one stable error shape.
		if joined := errors.Join(addrErrs...); joined != nil {
			p.notifyDialFailed(peerId, joined, dialStarted)
		} else {
			p.notifyDialFailed(peerId, err, dialStarted)
		}
		return
	}
	connPeerId, err := peer.CtxPeerId(mc.Context())
	if err != nil {
		_ = mc.Close()
		p.notifyDialFailed(peerId, err, dialStarted)
		return nil, err
	}
	if connPeerId != peerId {
		_ = mc.Close()
		p.notifyDialFailed(peerId, ErrPeerIdMismatched, dialStarted)
		return nil, ErrPeerIdMismatched
	}
	pr, err = peer.NewPeer(mc, p.server)
	if err != nil {
		_ = mc.Close()
		p.notifyDialFailed(peerId, err, dialStarted)
		return nil, err
	}
	protoVersion, _ := peer.CtxProtoVersion(mc.Context())
	// logAddr: an iroh ticket encodes the peer's relay and IP addresses,
	// which have no place in a status surface either
	p.observer.Notify(peerobserver.Event{
		Kind:         peerobserver.KindConnected,
		PeerId:       peerId,
		Addr:         stripScheme(logAddr(connAddr)),
		Scheme:       scheme(connAddr),
		ProtoVersion: protoVersion,
		Dur:          time.Since(dialStarted),
	})
	return pr, nil
}

func (p *peerService) notifyDialStarted(peerId string, addrCount int) {
	p.observer.Notify(peerobserver.Event{
		Kind:      peerobserver.KindDialStarted,
		PeerId:    peerId,
		AddrCount: addrCount,
	})
}

func (p *peerService) notifyDialFailed(peerId string, err error, dialStarted time.Time) {
	p.observer.Notify(peerobserver.Event{
		Kind:   peerobserver.KindDialFailed,
		PeerId: peerId,
		Err:    err,
		Dur:    time.Since(dialStarted),
	})
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
	globalDial := ctxIsGlobalDial(ctx)
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
		log.InfoCtx(ctx, "can't connect to host", zap.String("addr", logAddr(addr)), zap.Error(err))
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
	// notify before AddPeer so Connected always precedes the Closed that the
	// pool reports when the connection dies
	protoVersion, _ := peer.CtxProtoVersion(mc.Context())
	remoteAddr := mc.Addr()
	p.observer.Notify(peerobserver.Event{
		Kind:         peerobserver.KindConnected,
		PeerId:       pr.Id(),
		Addr:         stripScheme(remoteAddr),
		Scheme:       explicitScheme(remoteAddr),
		Inbound:      true,
		ProtoVersion: protoVersion,
	})
	if err = p.pool.AddPeer(context.Background(), pr); err != nil {
		_ = pr.Close()
		p.observer.Notify(peerobserver.Event{
			Kind:    peerobserver.KindClosed,
			PeerId:  pr.Id(),
			Inbound: true,
		})
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

// logAddr shortens iroh tickets: a ticket encodes the peer's relay and IP
// addresses, which have no place in shipped logs.
func logAddr(addr string) string {
	const keep = 12
	if scheme(addr) != transport.Iroh {
		return addr
	}
	if ticket := stripScheme(addr); len(ticket) > keep {
		return transport.Iroh + "://" + ticket[:keep] + "…"
	}
	return addr
}

func logAddrs(addrs []string) []string {
	out := make([]string, len(addrs))
	for i, addr := range addrs {
		out[i] = logAddr(addr)
	}
	return out
}

func scheme(addr string) string {
	if idx := strings.Index(addr, "://"); idx != -1 {
		return addr[:idx]
	}
	return transport.Yamux
}

// explicitScheme returns the scheme prefix of addr, or "" when it carries
// none — unlike scheme, it does not assume yamux for a bare address
func explicitScheme(addr string) string {
	if idx := strings.Index(addr, "://"); idx != -1 {
		return addr[:idx]
	}
	return ""
}

func stripScheme(addr string) string {
	if idx := strings.Index(addr, "://"); idx != -1 {
		return addr[idx+3:]
	}
	return addr
}
