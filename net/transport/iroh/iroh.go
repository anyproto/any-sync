package iroh

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/tmc/go-iroh/endpointticket"
	"github.com/tmc/go-iroh/iroh"
	"github.com/tmc/go-iroh/key"
	"github.com/tmc/go-iroh/netaddr"
	"github.com/tmc/go-iroh/relay"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
)

const (
	CName = "net.transport.iroh"
	// ALPN routes any-sync connections on the iroh endpoint.
	ALPN = "anysync/1"
)

var (
	log = logger.NewNamed(CName)

	ErrPeerIdMismatched = errors.New("iroh: ticket endpoint id does not match the expected peer id")
	ErrNotRunning       = errors.New("iroh: endpoint is not running")
)

func New() Iroh {
	return new(irohTransport)
}

// Iroh implements transport.Transport over an iroh endpoint: QUIC with
// relay fallback and hole punching, peers addressed by endpoint tickets.
// The endpoint identity is the device peer key (see PeerIdFromEndpointId).
type Iroh interface {
	transport.Transport
	app.ComponentRunnable
	// Ticket returns this endpoint's dialable ticket: relay addresses when
	// relays are configured, direct IP addresses otherwise. Empty until the
	// endpoint knows an address of that kind.
	Ticket() string
	// TicketUpdates wakes once per Ticket change (coalescing, one pending
	// signal); read Ticket after each wake-up.
	TicketUpdates() <-chan struct{}
	// SetIncomingFilter gates inbound connections by remote peer id before
	// the any-sync handshake runs; nil accepts everyone.
	SetIncomingFilter(f func(peerId string) bool)
}

type irohTransport struct {
	secure    secureservice.SecureService
	accepter  transport.Accepter
	conf      Config
	secretKey key.SecretKey
	relays    []netaddr.RelayURL

	ep        *iroh.Endpoint
	ticket    string
	updates   chan struct{}
	filter    func(peerId string) bool
	runCtx    context.Context
	runCancel context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
}

func (i *irohTransport) Init(a *app.App) (err error) {
	i.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	i.conf = a.MustComponent("config").(configGetter).GetIroh()
	if i.conf.DialTimeoutSec <= 0 {
		i.conf.DialTimeoutSec = 15
	}
	if i.conf.WriteTimeoutSec <= 0 {
		i.conf.WriteTimeoutSec = 10
	}
	if i.conf.CloseTimeoutSec <= 0 {
		i.conf.CloseTimeoutSec = 5
	}
	if i.conf.MaxStreams <= 0 {
		i.conf.MaxStreams = 128
	}
	for _, raw := range i.conf.RelayURLs {
		u, err := netaddr.ParseRelayURL(raw)
		if err != nil {
			return fmt.Errorf("iroh: relay url %q: %w", raw, err)
		}
		i.relays = append(i.relays, u)
	}
	acc := a.MustComponent(accountservice.CName).(accountservice.Service).Account()
	raw, err := acc.PeerKey.Raw()
	if err != nil {
		return err
	}
	if i.secretKey, err = key.SecretKeyFromEd25519(ed25519.PrivateKey(raw)); err != nil {
		return err
	}
	i.updates = make(chan struct{}, 1)
	i.runCtx, i.runCancel = context.WithCancel(context.Background())
	return nil
}

func (i *irohTransport) Name() string {
	return CName
}

func (i *irohTransport) SetAccepter(accepter transport.Accepter) {
	i.accepter = accepter
}

func (i *irohTransport) SetIncomingFilter(f func(peerId string) bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.filter = f
}

func (i *irohTransport) Run(ctx context.Context) (err error) {
	if i.accepter == nil {
		return fmt.Errorf("can't run service without accepter")
	}
	mode := relay.ModeDisabled()
	if len(i.relays) > 0 {
		mode = relay.ModeCustomURLs(i.relays...)
	}
	opts := []iroh.Option{
		iroh.WithSecretKey(i.secretKey),
		iroh.WithALPNs(ALPN),
		iroh.WithRelayMode(mode),
		iroh.WithTransportConfig(&iroh.QUICTransportConfig{
			KeepAlivePeriod:    time.Duration(i.conf.KeepAlivePeriodSec) * time.Second,
			MaxIdleTimeout:     time.Duration(i.conf.MaxIdleTimeoutSec) * time.Second,
			MaxIncomingStreams: i.conf.MaxStreams,
		}),
	}
	if i.conf.BindAddr != "" {
		ap, err := netip.ParseAddrPort(i.conf.BindAddr)
		if err != nil {
			return fmt.Errorf("iroh: bind addr %q: %w", i.conf.BindAddr, err)
		}
		opts = append(opts, iroh.WithBindAddr(ap))
	}
	ep, err := iroh.Bind(ctx, opts...)
	if err != nil {
		return err
	}
	i.mu.Lock()
	i.ep = ep
	i.mu.Unlock()
	i.wg.Add(2)
	go i.acceptLoop(ep)
	go i.watchAddr(ep)
	log.Info("iroh endpoint started", zap.String("endpointId", ep.ID().String()), zap.Stringer("localAddr", ep.LocalAddr()), zap.Int("relays", len(i.relays)))
	return nil
}

func (i *irohTransport) endpoint() *iroh.Endpoint {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.ep
}

func (i *irohTransport) Ticket() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.ticket
}

func (i *irohTransport) TicketUpdates() <-chan struct{} {
	return i.updates
}

func (i *irohTransport) setTicket(ticket string) {
	i.mu.Lock()
	changed := ticket != i.ticket
	i.ticket = ticket
	i.mu.Unlock()
	if !changed {
		return
	}
	log.Debug("iroh ticket changed", zap.String("ticket", ticket))
	select {
	case i.updates <- struct{}{}:
	default:
	}
}

// ticketFor keeps the address kind this endpoint is reachable through:
// relay addresses with relays configured, direct IPs otherwise (falling back
// to the bound socket address, which the address watcher omits for loopback
// binds). Relay tickets stay stable across network changes.
func (i *irohTransport) ticketFor(ep *iroh.Endpoint, addr netaddr.EndpointAddr) string {
	var keep []netaddr.TransportAddr
	for _, a := range addr.Addrs() {
		switch a.(type) {
		case netaddr.RelayAddr:
			if len(i.relays) > 0 {
				keep = append(keep, a)
			}
		case netaddr.IPAddr:
			if len(i.relays) == 0 {
				keep = append(keep, a)
			}
		}
	}
	if len(keep) == 0 && len(i.relays) == 0 {
		if local := ep.LocalAddr(); local.IsValid() && !local.Addr().IsUnspecified() {
			keep = append(keep, netaddr.IPAddr{Addr: local})
		}
	}
	if len(keep) == 0 {
		return ""
	}
	return endpointticket.Encode(netaddr.NewEndpointAddr(ep.ID(), keep...))
}

func (i *irohTransport) watchAddr(ep *iroh.Endpoint) {
	defer i.wg.Done()
	obs := ep.WatchAddr()
	for {
		addr, err := obs.Updated(i.runCtx)
		if err != nil {
			return
		}
		i.setTicket(i.ticketFor(ep, addr))
	}
}

func (i *irohTransport) Dial(ctx context.Context, addr string) (mc transport.MultiConn, err error) {
	ep := i.endpoint()
	if ep == nil {
		return nil, ErrNotRunning
	}
	target, err := endpointticket.Decode(addr)
	if err != nil {
		return nil, fmt.Errorf("iroh: ticket %q: %w", addr, err)
	}
	peerId, err := PeerIdFromEndpointId(target.ID)
	if err != nil {
		return nil, err
	}
	if expected, expErr := peer.CtxExpectedPeerId(ctx); expErr == nil && expected != peerId {
		return nil, ErrPeerIdMismatched
	}
	dialTimeout := time.Duration(i.conf.DialTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	conn, err := ep.Connect(ctx, target, ALPN)
	if err != nil {
		return nil, err
	}
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		_ = conn.CloseWithError(closeCodeHandshake, err.Error())
		return nil, err
	}
	defer func() {
		_ = stream.Close()
	}()
	cctx, err := i.secure.HandshakeOutbound(ctx, stream, peerId)
	if err != nil {
		_ = conn.CloseWithError(closeCodeHandshake, "outbound handshake failed")
		return nil, err
	}
	return newConn(cctx, conn, time.Duration(i.conf.CloseTimeoutSec)*time.Second, time.Duration(i.conf.WriteTimeoutSec)*time.Second), nil
}

func (i *irohTransport) acceptLoop(ep *iroh.Endpoint) {
	defer i.wg.Done()
	l := log.With(zap.String("endpointId", ep.ID().String()))
	l.Info("iroh listener started")
	defer l.Debug("iroh listener stopped")
	for {
		conn, err := ep.Accept(i.runCtx)
		if err != nil {
			if i.runCtx.Err() != nil || errors.Is(err, iroh.ErrEndpointClosed) {
				return
			}
			l.Warn("iroh accept error", zap.Error(err))
			select {
			case <-time.After(time.Second):
				continue
			case <-i.runCtx.Done():
				return
			}
		}
		go i.accept(conn)
	}
}

func (i *irohTransport) accept(conn *iroh.Conn) {
	l := log.With(zap.String("remoteId", conn.RemoteID().String()))
	peerId, err := PeerIdFromEndpointId(conn.RemoteID())
	if err != nil {
		l.Info("incoming connection with unusable endpoint id", zap.Error(err))
		_ = conn.CloseWithError(closeCodeRefused, "unusable endpoint id")
		return
	}
	i.mu.Lock()
	filter := i.filter
	i.mu.Unlock()
	if filter != nil && !filter(peerId) {
		l.Debug("incoming connection refused by filter", zap.String("peerId", peerId))
		_ = conn.CloseWithError(closeCodeRefused, "refused")
		return
	}
	ctx, cancel := context.WithTimeout(i.runCtx, time.Duration(i.conf.DialTimeoutSec)*time.Second)
	defer cancel()
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		l.Info("incoming connection handshake stream error", zap.Error(err))
		_ = conn.CloseWithError(closeCodeHandshake, "no handshake stream")
		return
	}
	defer func() {
		_ = stream.Close()
	}()
	cctx, err := i.secure.HandshakeInbound(ctx, stream, peerId)
	if err != nil {
		l.Info("incoming connection handshake error", zap.Error(err))
		_ = conn.CloseWithError(closeCodeHandshake, "inbound handshake failed")
		return
	}
	mc := newConn(cctx, conn, time.Duration(i.conf.CloseTimeoutSec)*time.Second, time.Duration(i.conf.WriteTimeoutSec)*time.Second)
	if err = i.accepter.Accept(mc); err != nil {
		l.Info("connection accept error", zap.Error(err))
	}
}

func (i *irohTransport) Close(ctx context.Context) (err error) {
	if i.runCancel != nil {
		i.runCancel()
	}
	if ep := i.endpoint(); ep != nil {
		err = ep.Shutdown(ctx)
	}
	i.wg.Wait()
	return err
}
