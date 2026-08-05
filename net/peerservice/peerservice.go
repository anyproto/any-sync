package peerservice

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
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

// dialStaggerInterval is the head start each dial candidate gets before
// the next one launches when the parallel dial is enabled (happy-eyeballs,
// RFC 8305 §5). The stagger bounds how long a silently-dropping addr
// (filtered port, blackholed route) can delay the rest of the list —
// sequential mode pays a full dial timeout per dead addr. Var for tests.
var dialStaggerInterval = 300 * time.Millisecond

func New() PeerService {
	return new(peerService)
}

type PeerService interface {
	Dial(ctx context.Context, peerId string) (pr peer.Peer, err error)
	SetPeerAddrs(peerId string, addrs []string)
	PreferQuic(prefer bool)
	// SetParallelDial toggles the staggered parallel dial at runtime
	// (same rollout surface as PreferQuic); the default comes from the
	// optional config getter, else sequential.
	SetParallelDial(enabled bool)
	transport.Accepter
	app.Component
}

// parallelDialConfigGetter is the optional config surface for the
// staggered parallel dial. Deployments whose config component
// implements it get the initial value from config; SetParallelDial
// overrides at runtime.
type parallelDialConfigGetter interface {
	GetParallelDial() bool
}

type peerService struct {
	yamux        transport.Transport
	quic         transport.Transport
	webtransport transport.Transport
	nodeConf     nodeconf.NodeConf
	peerAddrs    map[string][]string
	pool         pool.Pool
	server       server.DRPCServer
	preferQuic   bool
	parallelDial bool
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
	p.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	p.pool = a.MustComponent(pool.CName).(pool.Pool)
	p.server = a.MustComponent(server.CName).(server.DRPCServer)
	p.peerAddrs = map[string][]string{}
	p.localAddrs = newLocalAddrDetector()
	if c, ok := a.Component("config").(parallelDialConfigGetter); ok {
		p.parallelDial = c.GetParallelDial()
	}
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

func (p *peerService) SetParallelDial(enabled bool) {
	p.mu.Lock()
	p.parallelDial = enabled
	p.mu.Unlock()
}

func (p *peerService) Dial(ctx context.Context, peerId string) (pr peer.Peer, err error) {
	p.mu.RLock()
	preferQuic := p.preferQuic
	parallel := p.parallelDial
	addrs, err := p.getPeerAddrs(peerId)
	p.mu.RUnlock()
	if err != nil {
		return
	}

	// Pass expected peerId in context for transports that need it (e.g. WebTransport)
	ctx = peer.CtxWithExpectedPeerId(ctx, peerId)

	ordered := p.orderAddrs(ctx, addrs, preferQuic)
	log.DebugCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", ordered))

	cands := make([]dialCandidate, 0, len(ordered))
	for _, addr := range ordered {
		if tr := p.transport(scheme(addr)); tr != nil {
			cands = append(cands, dialCandidate{tr: tr, addr: addr})
		}
	}
	if len(cands) == 0 {
		return nil, ErrAddrsNotFound
	}
	mc, err := p.dialCandidates(ctx, cands, parallel)
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
// comes first either way.
func (p *peerService) orderAddrs(ctx context.Context, addrs []string, preferQuic bool) []string {
	type candidate struct {
		addr string
		rank int
	}
	candidates := make([]candidate, 0, len(addrs))
	for _, addr := range addrs {
		schemes := p.preferredSchemes(preferQuic && !p.localAddrs.isLocal(ctx, stripScheme(addr)))
		rank := slices.Index(schemes, scheme(addr))
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

// dialCandidate is one (transport, addr) attempt; candidates arrive in
// orderAddrs preference order.
type dialCandidate struct {
	tr   transport.Transport
	addr string // scheme-prefixed; stripped at dial time, kept raw for logs
}

// dialCandidates walks the candidates. Sequential mode (parallel=false,
// the default) launches the next candidate only after the previous one
// failed — the classic loop, one attempt in flight. Parallel mode races
// them happy-eyeballs style (RFC 8305 §5): each candidate gets
// dialStaggerInterval of head start before the next launches; a failure
// still hands off immediately. The first success wins and cancels the
// rest; a loser that completes its dial after the race is decided is
// closed by the background drain. All-failed returns the joined attempt
// errors in both modes.
//
// In parallel mode preference is a head start, not a guarantee: a
// preferred candidate that is alive but slower than the stagger interval
// can lose to a lower-preference one that connects faster — the
// happy-eyeballs trade-off. orderAddrs still decides launch order.
func (p *peerService) dialCandidates(ctx context.Context, cands []dialCandidate, parallel bool) (transport.MultiConn, error) {
	dctx, cancel := context.WithCancel(ctx)
	type dialResult struct {
		mc  transport.MultiConn
		err error
	}
	// Buffered to len(cands): every launched goroutine can deliver its
	// result without blocking even when nobody is left to read it.
	results := make(chan dialResult, len(cands))
	launched, completed := 0, 0
	launch := func() {
		c := cands[launched]
		launched++
		go func() {
			mc, err := c.tr.Dial(dctx, stripScheme(c.addr))
			if err != nil {
				log.InfoCtx(ctx, "can't connect to host", zap.String("addr", c.addr), zap.Error(err))
			}
			results <- dialResult{mc: mc, err: err}
		}()
	}
	// finish cancels the still-running losers and drains their results in
	// the background, closing any connection that completed its dial after
	// the race was already decided.
	finish := func(mc transport.MultiConn, err error) (transport.MultiConn, error) {
		cancel()
		if outstanding := launched - completed; outstanding > 0 {
			go func() {
				for i := 0; i < outstanding; i++ {
					if r := <-results; r.err == nil && r.mc != nil {
						log.DebugCtx(ctx, "dial race: closing runner-up")
						_ = r.mc.Close()
					}
				}
			}()
		}
		return mc, err
	}

	var errs []error
	var staggerC <-chan time.Time
	launch()
	if parallel {
		staggerC = time.After(dialStaggerInterval)
	}
	for {
		if launched == len(cands) {
			staggerC = nil
		}
		select {
		case <-dctx.Done():
			return finish(nil, dctx.Err())
		case <-staggerC:
			launch()
			staggerC = time.After(dialStaggerInterval)
		case r := <-results:
			completed++
			if r.err == nil {
				return finish(r.mc, nil)
			}
			errs = append(errs, r.err)
			if completed == len(cands) {
				return finish(nil, errors.Join(errs...))
			}
			if launched < len(cands) {
				launch()
				if parallel {
					staggerC = time.After(dialStaggerInterval)
				}
			}
		}
	}
}

func (p *peerService) transport(sch string) transport.Transport {
	switch sch {
	case transport.Quic:
		return p.quic
	case transport.Yamux:
		return p.yamux
	case transport.WebTransport:
		return p.webtransport
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
