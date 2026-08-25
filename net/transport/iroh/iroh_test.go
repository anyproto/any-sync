package iroh

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/go-iroh/endpointticket"
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
	fx := &fixture{
		irohTransport: New().(*irohTransport),
		ctrl:          gomock.NewController(t),
		acc:           &accounttest.AccountTestService{},
		accepter:      &testAccepter{mcs: make(chan transport.MultiConn, 100)},
		a:             new(app.App),
	}
	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(ctx)
	fx.mockNodeConf.EXPECT().Close(ctx)
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()
	fx.a.Register(fx.acc).Register(newTestConf()).Register(fx.mockNodeConf).Register(secureservice.New()).Register(fx.irohTransport).Register(fx.accepter)
	require.NoError(t, fx.a.Start(ctx))
	fx.peerId = fx.acc.Account().PeerId
	require.Eventually(t, func() bool { return fx.Ticket() != "" }, 5*time.Second, 10*time.Millisecond, "no ticket")
	return fx
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

func newTestConf() *testConf {
	return &testConf{testnodeconf.GenNodeConfig(1)}
}

type testConf struct {
	*testnodeconf.Config
}

func (c *testConf) GetIroh() Config {
	return Config{
		BindAddr:        "127.0.0.1:0",
		WriteTimeoutSec: 5,
		DialTimeoutSec:  5,
		CloseTimeoutSec: 2,
	}
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
