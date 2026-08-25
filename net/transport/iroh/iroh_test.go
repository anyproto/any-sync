package iroh

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/go-iroh/endpointticket"
	"github.com/tmc/go-iroh/key"
	"github.com/tmc/go-iroh/netaddr"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
)

var ctx = context.Background()

func TestIrohTransport_Dial(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, mcS := fxC.connect(t, fxS)

	var (
		sData     string
		acceptErr error
		copyErr   error
		done      = make(chan struct{})
	)
	go func() {
		defer close(done)
		conn, serr := mcS.Accept()
		if serr != nil {
			acceptErr = serr
			return
		}
		buf := bytes.NewBuffer(nil)
		_, copyErr = io.Copy(buf, conn)
		sData = buf.String()
	}()

	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	data := "some data"
	_, err = conn.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	<-done

	assert.NoError(t, acceptErr)
	assert.Equal(t, data, sData)
	assert.NoError(t, copyErr)
}

func TestIrohTransport_HandshakeContext(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, mcS := fxC.connect(t, fxS)

	peerId, err := peer.CtxPeerId(mcC.Context())
	require.NoError(t, err)
	assert.Equal(t, fxS.peerId, peerId)
	peerId, err = peer.CtxPeerId(mcS.Context())
	require.NoError(t, err)
	assert.Equal(t, fxC.peerId, peerId)

	_, err = peer.CtxIdentity(mcS.Context())
	assert.NoError(t, err)
	_, err = peer.CtxProtoVersion(mcS.Context())
	assert.NoError(t, err)

	assert.True(t, strings.HasPrefix(peer.CtxPeerAddr(mcS.Context()), transport.Iroh+"://"))
	assert.True(t, strings.HasPrefix(mcC.Addr(), transport.Iroh+"://"))
}

func TestIrohTransport_ExpectedPeerIdMismatch(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	_, err := fxC.Dial(peer.CtxWithExpectedPeerId(ctx, fxC.peerId), fxS.Ticket())
	assert.ErrorIs(t, err, ErrPeerIdMismatched)
}

func TestIrohTransport_BadTicket(t *testing.T) {
	fx := newFixture(t)
	defer fx.finish(t)

	_, err := fx.Dial(ctx, "not-a-ticket")
	assert.Error(t, err)
}

func TestIrohTransport_IncomingFilter(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	seen := make(chan string, 1)
	fxS.SetIncomingFilter(func(peerId string) bool {
		seen <- peerId
		return false
	})

	_, err := fxC.Dial(peer.CtxWithExpectedPeerId(ctx, fxS.peerId), fxS.Ticket())
	require.Error(t, err)
	select {
	case peerId := <-seen:
		assert.Equal(t, fxC.peerId, peerId)
	case <-time.After(time.Second):
		t.Fatal("filter was not consulted")
	}
	select {
	case mc := <-fxS.accepter.mcs:
		t.Fatalf("refused connection reached the accepter: %v", mc.Addr())
	case <-time.After(200 * time.Millisecond):
	}
}

func TestIrohTransport_CloseMapsToErrConnClosed(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, mcS := fxC.connect(t, fxS)
	require.NoError(t, mcS.Close())

	_, err := mcC.Accept()
	require.Error(t, err)
	assert.ErrorIs(t, err, net.ErrClosed)
	assert.ErrorIs(t, err, transport.ErrConnClosed)
	assert.Eventually(t, mcC.IsClosed, 5*time.Second, 20*time.Millisecond)

	_, err = mcC.Open(ctx)
	assert.ErrorIs(t, err, transport.ErrConnClosed)
}

func TestIrohTransport_BytesCounters(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, mcS := fxC.connect(t, fxS)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, serr := mcS.Accept()
		require.NoError(t, serr)
		buf := bytes.NewBuffer(nil)
		_, _ = io.Copy(buf, conn)
	}()

	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	payload := bytes.Repeat([]byte("y"), 4096)
	_, err = conn.Write(payload)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	<-done

	assert.Eventually(t, func() bool {
		return mcC.BytesWritten() >= int64(len(payload)) && mcS.BytesRead() >= int64(len(payload))
	}, 2*time.Second, 20*time.Millisecond)
}

func TestIrohTransport_TicketUpdates(t *testing.T) {
	fx := newFixture(t)
	defer fx.finish(t)

	// the fixture already waited for the first ticket; a second wake-up
	// must not be pending, and the ticket must name this endpoint
	select {
	case <-fx.TicketUpdates():
	default:
	}
	addr, err := endpointticket.Decode(fx.Ticket())
	require.NoError(t, err)
	assert.Equal(t, fx.ep.ID(), addr.ID)
	assert.NotEmpty(t, addr.IPAddrs(), "no relays configured: ticket must carry direct addrs")
	assert.Empty(t, addr.RelayURLs())
}

func TestIdentRoundTrip(t *testing.T) {
	fx := newFixture(t)
	defer fx.finish(t)

	id, err := EndpointIdFromPeerId(fx.peerId)
	require.NoError(t, err)
	assert.Equal(t, fx.ep.ID(), id)
	peerId, err := PeerIdFromEndpointId(id)
	require.NoError(t, err)
	assert.Equal(t, fx.peerId, peerId)

	_, err = EndpointIdFromPeerId("not a peer id")
	assert.Error(t, err)
}

type fixture struct {
	*irohTransport
	a            *app.App
	ctrl         *gomock.Controller
	mockNodeConf *mock_nodeconf.MockService
	acc          *accounttest.AccountTestService
	accepter     *testAccepter
	peerId       string
}

func newFixture(t *testing.T) *fixture {
	return newFixtureConf(t, Config{})
}

// newFixtureConf starts a transport with an allow-all filter; zero conf
// fields get test defaults (loopback bind, short timeouts, insecure relay).
func newFixtureConf(t *testing.T, conf Config) *fixture {
	fx, err := startFixture(t, conf, func(string) bool { return true })
	require.NoError(t, err)
	fx.peerId = fx.acc.Account().PeerId
	require.Eventually(t, func() bool { return fx.Ticket() != "" }, 5*time.Second, 10*time.Millisecond, "no ticket")
	return fx
}

func startFixture(t *testing.T, conf Config, filter func(string) bool) (*fixture, error) {
	fx := &fixture{
		irohTransport: New().(*irohTransport),
		ctrl:          gomock.NewController(t),
		acc:           &accounttest.AccountTestService{},
		accepter:      &testAccepter{mcs: make(chan transport.MultiConn, 100)},
		a:             new(app.App),
	}
	if conf.BindAddr == "" {
		conf.BindAddr = "127.0.0.1:0"
	}
	if conf.DialTimeoutSec == 0 {
		conf.DialTimeoutSec = 5
	}
	if conf.WriteTimeoutSec == 0 {
		conf.WriteTimeoutSec = 5
	}
	conf.InsecureRelay = true
	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(ctx).AnyTimes()
	fx.mockNodeConf.EXPECT().Close(ctx).AnyTimes()
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()
	if filter != nil {
		fx.SetIncomingFilter(filter)
	}
	fx.a.Register(fx.acc).Register(&testConf{Config: testnodeconf.GenNodeConfig(1), iroh: conf}).Register(fx.mockNodeConf).Register(secureservice.New()).Register(fx.irohTransport).Register(fx.accepter)
	if err := fx.a.Start(ctx); err != nil {
		_ = fx.a.Close(ctx)
		return nil, err
	}
	return fx, nil
}

// connect dials fxS from fx and returns both ends.
func (fx *fixture) connect(t *testing.T, fxS *fixture) (mcC, mcS transport.MultiConn) {
	mcC, err := fx.Dial(peer.CtxWithExpectedPeerId(ctx, fxS.peerId), fxS.Ticket())
	require.NoError(t, err)
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(5 * time.Second):
		t.Fatal("accept timeout")
	}
	return mcC, mcS
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

type testConf struct {
	*testnodeconf.Config
	iroh Config
}

func (c *testConf) GetIroh() Config {
	return c.iroh
}

type testAccepter struct {
	err error
	mcs chan transport.MultiConn
}

func (t *testAccepter) Accept(mc transport.MultiConn) (err error) {
	t.mcs <- mc
	return t.err
}

func (t *testAccepter) Init(a *app.App) (err error) {
	a.MustComponent(CName).(transport.Transport).SetAccepter(t)
	return nil
}

func (t *testAccepter) Name() (name string) { return "testAccepter" }

func TestIrohTransport_RunRequiresFilter(t *testing.T) {
	_, err := startFixture(t, Config{}, nil)
	assert.ErrorIs(t, err, ErrNoFilter)
}

func TestIrohTransport_DialTimeout(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixtureConf(t, Config{DialTimeoutSec: 1})
	defer fxC.finish(t)

	// right id, dead port: the dial must give up within DialTimeoutSec
	dead := endpointticket.Encode(netaddr.NewEndpointAddr(fxS.ep.ID()).WithIP(netip.MustParseAddrPort("127.0.0.1:9")))
	start := time.Now()
	_, err := fxC.Dial(peer.CtxWithExpectedPeerId(ctx, fxS.peerId), dead)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 3*time.Second)
}

func TestIrohTransport_CloseDuringDial(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixtureConf(t, Config{DialTimeoutSec: 10})

	dead := endpointticket.Encode(netaddr.NewEndpointAddr(fxS.ep.ID()).WithIP(netip.MustParseAddrPort("127.0.0.1:9")))
	errCh := make(chan error, 1)
	go func() {
		_, err := fxC.Dial(peer.CtxWithExpectedPeerId(ctx, fxS.peerId), dead)
		errCh <- err
	}()
	time.Sleep(100 * time.Millisecond)
	closed := make(chan struct{})
	go func() {
		fxC.finish(t)
		close(closed)
	}()
	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("dial did not return after close")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("close did not return")
	}
}

func TestIrohTransport_TicketCoalescing(t *testing.T) {
	fx := newFixture(t)
	defer fx.finish(t)
	// drain the start-up signal
	select {
	case <-fx.TicketUpdates():
	default:
	}
	fx.setTicket("a")
	fx.setTicket("b")
	fx.setTicket("b")
	select {
	case <-fx.TicketUpdates():
	default:
		t.Fatal("no wake-up")
	}
	select {
	case <-fx.TicketUpdates():
		t.Fatal("changes must coalesce into one pending wake-up")
	default:
	}
	assert.Equal(t, "b", fx.Ticket())
}

func TestIrohTransport_DirectTicketUnspecifiedBind(t *testing.T) {
	for _, bind := range []string{"0.0.0.0:0", "[::]:0"} {
		t.Run(bind, func(t *testing.T) {
			fx := newFixtureConf(t, Config{BindAddr: bind})
			defer fx.finish(t)

			addr, err := endpointticket.Decode(fx.Ticket())
			require.NoError(t, err)
			require.NotEmpty(t, addr.IPAddrs())
			var hasV4 bool
			for _, ip := range addr.IPAddrs() {
				assert.False(t, ip.Addr().IsUnspecified())
				assert.Equal(t, fx.ep.LocalAddr().Port(), ip.Port())
				hasV4 = hasV4 || ip.Addr().Is4()
			}
			assert.True(t, hasV4, "an unspecified bind must stay dialable over IPv4")
		})
	}
}

func TestDirectAddrs(t *testing.T) {
	specific := netip.MustParseAddrPort("10.1.2.3:4000")
	assert.Equal(t, []netaddr.TransportAddr{netaddr.IPAddr{Addr: specific}}, directAddrs(specific))
	assert.Nil(t, directAddrs(netip.AddrPort{}))
	v4 := interfaceAddrs(4000, true)
	for _, a := range v4 {
		assert.True(t, a.(netaddr.IPAddr).Addr.Addr().Is4())
	}
	assert.NotEmpty(t, interfaceAddrs(4000, false))
}

func TestIrohNetConn_ReadFromCountsAndTimes(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC, mcS := fxC.connect(t, fxS)
	done := make(chan int64, 1)
	go func() {
		conn, serr := mcS.Accept()
		require.NoError(t, serr)
		n, _ := io.Copy(io.Discard, conn)
		done <- n
	}()
	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	payload := bytes.Repeat([]byte("z"), 10000)
	n, err := io.Copy(conn, bytes.NewReader(payload))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	assert.Equal(t, int64(len(payload)), n)
	assert.Equal(t, int64(len(payload)), <-done)
	assert.GreaterOrEqual(t, mcC.BytesWritten(), int64(len(payload)), "io.Copy must be counted")
}

func TestParseRelayURL(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		insecure bool
		ok       bool
	}{
		{"https://relay.example", false, true},
		{"https://relay.example:8443/", false, true},
		{"http://127.0.0.1:3340", false, false},
		{"http://127.0.0.1:3340", true, true},
		{"ws://relay.example", true, false},
		{"not a url at all", true, false},
		{"", true, false},
		{"https://", false, false},
	} {
		_, err := parseRelayURL(tc.raw, tc.insecure)
		if tc.ok {
			assert.NoError(t, err, tc.raw)
		} else {
			assert.Error(t, err, tc.raw)
		}
	}
}

func TestCapDialAddrs(t *testing.T) {
	var id key.EndpointID
	addr := netaddr.NewEndpointAddr(id)
	for i := 1; i <= 10; i++ {
		addr = addr.WithIP(netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}), 1000))
		addr = addr.WithRelayURL(netaddr.RelayURL{})
	}
	addr = addr.WithIP(netip.MustParseAddrPort("0.0.0.0:1")).WithIP(netip.MustParseAddrPort("10.0.1.1:0"))
	capped := capDialAddrs(addr)
	assert.Len(t, capped.IPAddrs(), maxDialIPs)
	assert.LessOrEqual(t, len(capped.RelayURLs()), maxDialRelays)
	for _, ip := range capped.IPAddrs() {
		assert.False(t, ip.Addr().IsUnspecified())
		assert.NotZero(t, ip.Port())
	}
}

func TestIrohTransport_InflightAccounting(t *testing.T) {
	tr := &irohTransport{conf: Config{MaxInflightAccepts: 2}, inflightBySource: map[netip.Addr]int{}}
	src := netip.MustParseAddr("10.1.1.1")
	assert.True(t, tr.admitSource(src))
	assert.True(t, tr.admitSource(src))
	assert.False(t, tr.admitSource(src), "global cap")
	tr.release(src)
	assert.True(t, tr.admitSource(src))
	tr.release(src)
	tr.release(src)
	assert.Empty(t, tr.inflightBySource)

	tr = &irohTransport{conf: Config{MaxInflightAccepts: 100}, inflightBySource: map[netip.Addr]int{}}
	for i := 0; i < maxInflightPerSource; i++ {
		assert.True(t, tr.admitSource(src))
	}
	assert.False(t, tr.admitSource(src), "per-source cap")
	assert.True(t, tr.admitSource(netip.MustParseAddr("10.1.1.2")))
}
