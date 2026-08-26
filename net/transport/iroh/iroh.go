package iroh

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/tmc/go-iroh/endpointticket"
	goiroh "github.com/tmc/go-iroh/iroh"
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
	CName = transport.IrohCName
	// ALPN routes any-sync connections on the iroh endpoint.
	ALPN = "anysync/1"

	// maxDirectAddrs bounds the interface addresses a direct (no relay)
	// ticket carries.
	maxDirectAddrs = 8
	// maxDialRelays / maxDialIPs cap the addresses taken from a peer's
	// ticket: a ticket is remote input and could otherwise point dials at
	// thousands of hosts.
	maxDialRelays = 2
	maxDialIPs    = 4
	// maxInflightPerSource bounds in-flight inbound connections per source
	// address: the UDP source for direct paths, the relay-mapped address
	// (one per remote endpoint id) for relayed ones.
	maxInflightPerSource = 8
)

var (
	log = logger.NewNamed(CName)

	ErrPeerIdMismatched = errors.New("iroh: ticket endpoint id does not match the expected peer id")
	ErrNotRunning       = errors.New("iroh: endpoint is not running")
	ErrNoFilter         = errors.New("iroh: incoming filter is required")
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
	// Ticket returns this endpoint's dialable ticket: the relay address once
	// the home-relay session is up when relays are configured, direct IP
	// addresses otherwise. Empty until then.
	Ticket() string
	// TicketUpdates wakes once per Ticket change (coalescing, one pending
	// signal); read Ticket after each wake-up.
	TicketUpdates() <-chan struct{}
	// RelayConnected reports whether the home-relay session is up. Always
	// false without relays.
	RelayConnected() bool
	// SetIncomingFilter gates inbound connections by remote peer id before
	// the any-sync handshake runs. The endpoint is reachable from the whole
	// internet through the relay, so Run refuses to start without one.
	SetIncomingFilter(f func(peerId string) bool)
	// SetHandshakeFilter gates inbound connections once the any-sync
	// handshake has proven the remote's identity; nil admits every peer the
	// incoming filter let through.
	SetHandshakeFilter(f func(peerId string, identity []byte) bool)
}

// TicketForPeer returns the relay-only ticket of a peer reachable at
// relayURL: the endpoint id derives from the peer id, so a peer's address is
// its peer id plus its home relay.
func TicketForPeer(peerId, relayURL string) (string, error) {
	id, err := EndpointIdFromPeerId(peerId)
	if err != nil {
		return "", err
	}
	u, err := netaddr.ParseRelayURL(relayURL)
	if err != nil {
		return "", fmt.Errorf("iroh: relay url %q: %w", relayURL, err)
	}
	return endpointticket.Encode(netaddr.NewEndpointAddr(id).WithRelayURL(u)), nil
}

// PeerIdFromTicket returns the any-sync peer id a ticket belongs to.
func PeerIdFromTicket(ticket string) (string, error) {
	addr, err := endpointticket.Decode(ticket)
	if err != nil {
		return "", err
	}
	return PeerIdFromEndpointId(addr.ID)
}

type irohTransport struct {
	secure    secureservice.SecureService
	accepter  transport.Accepter
	conf      Config
	secretKey key.SecretKey
	relays    []netaddr.RelayURL

	ep        *goiroh.Endpoint
	ticket    string
	updates   chan struct{}
	filter    func(peerId string) bool
	hsFilter  func(peerId string, identity []byte) bool
	runCtx    context.Context
	runCancel context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex

	inflight         int
	inflightBySource map[netip.Addr]int
	inflightMu       sync.Mutex
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
	if i.conf.MaxInflightAccepts <= 0 {
		i.conf.MaxInflightAccepts = 64
	}
	if i.conf.PeerTTLSec <= 0 {
		i.conf.PeerTTLSec = 1800
	}
	if i.conf.MaxStreams <= 0 {
		i.conf.MaxStreams = 128
	}
	if i.conf.KeepAlivePeriodSec <= 0 {
		i.conf.KeepAlivePeriodSec = 25
	}
	if i.conf.MaxIdleTimeoutSec > 0 && i.conf.KeepAlivePeriodSec >= i.conf.MaxIdleTimeoutSec {
		return fmt.Errorf("iroh: keepAlivePeriodSec %d must be below maxIdleTimeoutSec %d", i.conf.KeepAlivePeriodSec, i.conf.MaxIdleTimeoutSec)
	}
	for _, raw := range i.conf.RelayURLs {
		u, err := parseRelayURL(raw, i.conf.InsecureRelay)
		if err != nil {
			return err
		}
		i.relays = append(i.relays, u)
	}
	i.inflightBySource = map[netip.Addr]int{}
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

// parseRelayURL accepts https URLs with a host (http only when insecure is
// set: the relay link is then a plaintext WebSocket). ParseRelayURL itself
// normalizes almost anything, and a bad entry would sit in the relay map as
// a home relay that never connects.
func parseRelayURL(raw string, insecure bool) (netaddr.RelayURL, error) {
	u, err := netaddr.ParseRelayURL(raw)
	if err != nil {
		return netaddr.RelayURL{}, fmt.Errorf("iroh: relay url %q: %w", raw, err)
	}
	switch u.URL().Scheme {
	case "https":
	case "http":
		if !insecure {
			return netaddr.RelayURL{}, fmt.Errorf("iroh: relay url %q: http needs insecureRelay", raw)
		}
	default:
		return netaddr.RelayURL{}, fmt.Errorf("iroh: relay url %q: scheme must be https", raw)
	}
	if u.Host() == "" {
		return netaddr.RelayURL{}, fmt.Errorf("iroh: relay url %q: missing host", raw)
	}
	return u, nil
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

func (i *irohTransport) SetHandshakeFilter(f func(peerId string, identity []byte) bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.hsFilter = f
}

func (i *irohTransport) Run(ctx context.Context) (err error) {
	if i.accepter == nil {
		return fmt.Errorf("can't run service without accepter")
	}
	i.mu.Lock()
	hasFilter := i.filter != nil
	i.mu.Unlock()
	if !hasFilter {
		return ErrNoFilter
	}
	mode := relay.ModeDisabled()
	opts := []goiroh.Option{
		goiroh.WithSecretKey(i.secretKey),
		goiroh.WithALPNs(ALPN),
		goiroh.WithTransportConfig(&goiroh.QUICTransportConfig{
			KeepAlivePeriod:    time.Duration(i.conf.KeepAlivePeriodSec) * time.Second,
			MaxIdleTimeout:     time.Duration(i.conf.MaxIdleTimeoutSec) * time.Second,
			InitialPacketSize:  i.conf.InitialPacketSize,
			MaxIncomingStreams: i.conf.MaxStreams,
		}),
	}
	if len(i.relays) > 0 {
		mode = relay.ModeCustomURLs(i.relays...)
		// net reports learn the reflexive address behind NAT: without them
		// hole punching has no candidate and every connection stays relayed
		opts = append(opts, goiroh.WithNetReport())
	} else {
		log.Warn("iroh: no relay configured, endpoint is reachable by direct addresses only")
	}
	opts = append(opts, goiroh.WithRelayMode(mode))
	if i.conf.BindAddr != "" {
		ap, err := netip.ParseAddrPort(i.conf.BindAddr)
		if err != nil {
			return fmt.Errorf("iroh: bind addr %q: %w", i.conf.BindAddr, err)
		}
		opts = append(opts, goiroh.WithBindAddr(ap))
	}
	ep, err := goiroh.Bind(ctx, opts...)
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

func (i *irohTransport) endpoint() *goiroh.Endpoint {
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

func (i *irohTransport) RelayConnected() bool {
	ep := i.endpoint()
	if ep == nil || len(i.relays) == 0 {
		return false
	}
	st := ep.HomeRelayStatus().Current()
	return st != nil && st.IsConnected()
}

func (i *irohTransport) setTicket(ticket string) {
	i.mu.Lock()
	changed := ticket != i.ticket
	i.ticket = ticket
	i.mu.Unlock()
	if !changed {
		return
	}
	log.Debug("iroh ticket changed")
	select {
	case i.updates <- struct{}{}:
	default:
	}
}

// ticketFor keeps the address kind this endpoint is reachable through:
// relay addresses with relays configured, direct IPs otherwise. Relay
// tickets stay stable across network changes.
func (i *irohTransport) ticketFor(ep *goiroh.Endpoint, addr netaddr.EndpointAddr) string {
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
		keep = directAddrs(ep.LocalAddr())
	}
	if len(keep) == 0 {
		return ""
	}
	return endpointticket.Encode(netaddr.NewEndpointAddr(ep.ID(), keep...))
}

// directAddrs lists the addresses a peer can dial the bound socket on. The
// endpoint's own address watcher reports no candidates for loopback or
// unspecified binds, so a specific bind is used as is and an unspecified
// one expands to the interface addresses: both families for the dual-stack
// [::] bind, IPv4 only for 0.0.0.0.
func directAddrs(local netip.AddrPort) []netaddr.TransportAddr {
	if !local.IsValid() {
		return nil
	}
	if !local.Addr().IsUnspecified() {
		return []netaddr.TransportAddr{netaddr.IPAddr{Addr: local}}
	}
	return interfaceAddrs(local.Port(), local.Addr().Unmap().Is4())
}

func interfaceAddrs(port uint16, v4Only bool) []netaddr.TransportAddr {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []netaddr.TransportAddr
	var loopback []netaddr.TransportAddr
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipNet.IP)
			if !ok {
				continue
			}
			ip = ip.Unmap()
			if (v4Only && !ip.Is4()) || ip.IsLinkLocalUnicast() {
				continue
			}
			ta := netaddr.IPAddr{Addr: netip.AddrPortFrom(ip, port)}
			if ip.IsLoopback() {
				loopback = append(loopback, ta)
			} else if len(out) < maxDirectAddrs {
				out = append(out, ta)
			}
		}
	}
	if len(out) == 0 {
		return loopback
	}
	return out
}

func (i *irohTransport) watchAddr(ep *goiroh.Endpoint) {
	defer i.wg.Done()
	// a relay ticket is dialable only after the relay session is up: peers
	// that read it earlier would dial into a relay that drops their frames
	if len(i.relays) > 0 {
		if err := ep.Online(i.runCtx); err != nil {
			if i.runCtx.Err() == nil {
				log.Warn("iroh home relay never connected, no ticket", zap.Error(err))
			}
			return
		}
		log.Info("iroh home relay connected", zap.String("endpointId", ep.ID().String()))
	}
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
		return nil, fmt.Errorf("iroh: bad ticket: %w", err)
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
	// the relay drops frames for endpoints it has not registered yet, so a
	// relay dial waits for our own home-relay session first (no-op once up)
	if len(i.relays) > 0 {
		if err = ep.Online(ctx); err != nil {
			return nil, fmt.Errorf("iroh: home relay not connected: %w", err)
		}
	}
	conn, err := ep.Connect(ctx, capDialAddrs(target), ALPN)
	if err != nil {
		return nil, err
	}
	// a resumed session returns before the TLS handshake completes (0-RTT);
	// the any-sync handshake must not go out as replayable early data and
	// the remote id is verified only once the handshake is done
	select {
	case <-conn.HandshakeComplete():
	case <-conn.Context().Done():
		return nil, fmt.Errorf("iroh: connection closed during handshake: %w", context.Cause(conn.Context()))
	case <-ctx.Done():
		_ = conn.CloseWithError(closeCodeHandshake, "handshake timeout")
		return nil, ctx.Err()
	}
	if conn.RemoteID() != target.ID {
		_ = conn.CloseWithError(closeCodeHandshake, "unexpected endpoint id")
		return nil, ErrPeerIdMismatched
	}
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		_ = conn.CloseWithError(closeCodeHandshake, "no handshake stream")
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
	return i.newConn(cctx, conn), nil
}

func (i *irohTransport) newConn(cctx context.Context, conn *goiroh.Conn) transport.MultiConn {
	cctx = peer.CtxWithTTL(cctx, time.Duration(i.conf.PeerTTLSec)*time.Second)
	return newConn(cctx, conn, time.Duration(i.conf.CloseTimeoutSec)*time.Second, time.Duration(i.conf.WriteTimeoutSec)*time.Second)
}

// capDialAddrs rebuilds a ticket's address with bounded relay and IP
// candidate counts, dropping addresses a dial must not target.
func capDialAddrs(addr netaddr.EndpointAddr) netaddr.EndpointAddr {
	var relays, ips []netaddr.TransportAddr
	for _, a := range addr.Addrs() {
		switch ta := a.(type) {
		case netaddr.RelayAddr:
			if len(relays) < maxDialRelays {
				relays = append(relays, ta)
			}
		case netaddr.IPAddr:
			ip := ta.Addr.Addr().Unmap()
			if len(ips) < maxDialIPs && ip.IsValid() && !ip.IsUnspecified() && !ip.IsMulticast() && ta.Addr.Port() != 0 {
				ips = append(ips, ta)
			}
		}
	}
	return netaddr.NewEndpointAddr(addr.ID, append(ips, relays...)...)
}

// acceptLoop takes incoming attempts before their QUIC handshake completes
// and finishes each one on its own goroutine, so a slow or failing peer
// never holds the others back.
func (i *irohTransport) acceptLoop(ep *goiroh.Endpoint) {
	defer i.wg.Done()
	l := log.With(zap.String("endpointId", ep.ID().String()))
	l.Info("iroh listener started")
	defer l.Debug("iroh listener stopped")
	for {
		in, err := ep.AcceptIncoming(i.runCtx)
		if err != nil {
			if i.runCtx.Err() != nil || errors.Is(err, goiroh.ErrEndpointClosed) || errors.Is(err, net.ErrClosed) {
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
		source, ok := i.admit(in)
		if !ok {
			in.Refuse()
			continue
		}
		i.wg.Add(1)
		go i.accept(in, source)
	}
}

// admit reserves an in-flight slot for an incoming attempt, refusing it
// before any handshake work once the global or per-source bound is hit.
// Only the global bound holds against a flood of fresh endpoint keys.
func (i *irohTransport) admit(in *goiroh.Incoming) (source netip.Addr, ok bool) {
	if ua, isUDP := in.RemoteAddr().(*net.UDPAddr); isUDP {
		if ip, valid := netip.AddrFromSlice(ua.IP); valid {
			source = ip.Unmap()
		}
	}
	return source, i.admitSource(source)
}

func (i *irohTransport) admitSource(source netip.Addr) bool {
	i.inflightMu.Lock()
	defer i.inflightMu.Unlock()
	if i.inflight >= i.conf.MaxInflightAccepts {
		log.Debug("incoming connection refused: too many in flight", zap.Int("inflight", i.inflight))
		return false
	}
	if source.IsValid() && i.inflightBySource[source] >= maxInflightPerSource {
		log.Debug("incoming connection refused: too many from source", zap.Stringer("source", source))
		return false
	}
	i.inflight++
	if source.IsValid() {
		i.inflightBySource[source]++
	}
	return true
}

func (i *irohTransport) release(source netip.Addr) {
	i.inflightMu.Lock()
	defer i.inflightMu.Unlock()
	i.inflight--
	if source.IsValid() {
		if i.inflightBySource[source] <= 1 {
			delete(i.inflightBySource, source)
		} else {
			i.inflightBySource[source]--
		}
	}
}

func (i *irohTransport) accept(in *goiroh.Incoming, source netip.Addr) {
	defer i.wg.Done()
	defer i.release(source)
	ctx, cancel := context.WithTimeout(i.runCtx, time.Duration(i.conf.DialTimeoutSec)*time.Second)
	defer cancel()
	accepting, err := in.Accept()
	if err != nil {
		log.Debug("incoming connection not accepted", zap.Error(err))
		return
	}
	conn, err := accepting.Connection(ctx)
	if err != nil {
		log.Debug("incoming connection handshake failed", zap.Error(err), zap.Stringer("remoteAddr", accepting.RemoteAddr()))
		return
	}
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
	i.mu.Lock()
	hsFilter := i.hsFilter
	i.mu.Unlock()
	if hsFilter != nil {
		identity, _ := peer.CtxIdentity(cctx)
		if !hsFilter(peerId, identity) {
			l.Debug("incoming connection refused after handshake", zap.String("peerId", peerId))
			_ = conn.CloseWithError(closeCodeRefused, "refused")
			return
		}
	}
	if i.runCtx.Err() != nil {
		_ = conn.CloseWithError(closeCodeNormal, "closing")
		return
	}
	mc := i.newConn(cctx, conn)
	if err = i.accepter.Accept(mc); err != nil {
		l.Info("connection accept error", zap.Error(err))
		_ = mc.Close()
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
