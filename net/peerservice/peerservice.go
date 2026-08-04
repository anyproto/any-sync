package peerservice

import (
	"context"
	"errors"
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
// the next one launches in parallel (happy-eyeballs, RFC 8305 §5). A
// candidate that fails fast hands off immediately, so a network where
// every addr answers keeps the old sequential behavior; the stagger only
// bounds how long a silently-dropping addr (filtered port, blackholed
// route) can delay the rest of the list — previously a full dial timeout
// per dead addr, serialized across every addr and scheme. Var for tests.
var dialStaggerInterval = 300 * time.Millisecond

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
	nodeConf     nodeconf.NodeConf
	peerAddrs    map[string][]string
	pool         pool.Pool
	server       server.DRPCServer
	preferQuic   bool
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
	return nil
}

func (p *peerService) preferredSchemes() []string {
	var schemes []string
	if p.preferQuic {
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

func (p *peerService) Dial(ctx context.Context, peerId string) (pr peer.Peer, err error) {
	p.mu.RLock()
	schemes := p.preferredSchemes()
	addrs, err := p.getPeerAddrs(peerId)
	if err != nil {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	// Pass expected peerId in context for transports that need it (e.g. WebTransport)
	ctx = peer.CtxWithExpectedPeerId(ctx, peerId)

	log.DebugCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", addrs))

	var cands []dialCandidate
	for _, sch := range schemes {
		tr := p.transportFor(sch)
		if tr == nil {
			continue
		}
		for _, addr := range addrs {
			if scheme(addr) != sch {
				continue
			}
			cands = append(cands, dialCandidate{tr: tr, addr: addr})
		}
	}
	if len(cands) == 0 {
		return nil, ErrAddrsNotFound
	}
	mc, err := p.dialStaggered(ctx, cands)
	if err != nil {
		return nil, err
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
	return peer.NewPeer(mc, p.server)
}

func (p *peerService) transportFor(sch string) transport.Transport {
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

// dialCandidate is one (transport, addr) attempt; candidates are ordered
// by scheme preference first, addr list order second — the same order the
// old sequential loop tried them in.
type dialCandidate struct {
	tr   transport.Transport
	addr string // scheme-prefixed; stripped at dial time, kept raw for logs
}

// dialStaggered races the candidates: each gets dialStaggerInterval of
// head start before the next launches; a failure hands off to the next
// candidate immediately. The first success wins and cancels the rest;
// a loser that completes its dial after the race is decided is closed
// by the background drain. All-failed returns the joined attempt errors.
//
// Preference is a head start, not a guarantee: a preferred candidate
// that is alive but slower than the stagger interval can lose to a
// lower-preference one that connects faster — the happy-eyeballs
// trade-off. PreferQuic still decides launch order.
func (p *peerService) dialStaggered(ctx context.Context, cands []dialCandidate) (transport.MultiConn, error) {
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
						_ = r.mc.Close()
					}
				}
			}()
		}
		return mc, err
	}

	var errs []error
	launch()
	staggerC := time.After(dialStaggerInterval)
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
				staggerC = time.After(dialStaggerInterval)
			}
		}
	}
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
