package peerservice

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/mock_transport"
	"github.com/anyproto/any-sync/net/transport/quic"
	webtransportpkg "github.com/anyproto/any-sync/net/transport/webtransport"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var ctx = context.Background()

func TestPeerService_Dial(t *testing.T) {
	// public (non-local) addrs: the global preferQuic order applies
	var addrs = []string{
		"yamux://203.0.113.1:1111",
		"quic://203.0.113.1:1112",
	}
	t.Run("prefer yamux", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("prefer quic", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)

		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("first failed", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)

		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, fmt.Errorf("test"))
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("peerId mismatched", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId+"not valid"), nil)

		p, err := fx.Dial(ctx, peerId)
		assert.EqualError(t, err, ErrPeerIdMismatched.Error())
		assert.Nil(t, p)
	})
	t.Run("custom addr", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.SetPeerAddrs(peerId, addrs)
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(nil, false)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("addr without scheme", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"127.0.0.1:1111"}, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
}

func TestPeerService_DialLocalAddrs(t *testing.T) {
	var localAddrs = []string{
		"quic://192.168.1.5:1112",
		"yamux://192.168.1.5:1111",
	}
	t.Run("local addr prefers yamux even when quic preferred", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(localAddrs, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "192.168.1.5:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("local addr falls back to quic when yamux fails", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(localAddrs, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "192.168.1.5:1111").Return(nil, fmt.Errorf("connection refused"))
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "192.168.1.5:1112").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("hostname resolving to local addr prefers yamux", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		var peerId = "p1"

		fx.setResolver(func(_ context.Context, host string) ([]net.IPAddr, error) {
			require.Equal(t, "any-sync-node-1", host)
			return []net.IPAddr{{IP: net.ParseIP("172.18.0.5")}}, nil
		})
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"quic://any-sync-node-1:1112",
			"yamux://any-sync-node-1:1111",
		}, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "any-sync-node-1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("unresolved hostname keeps global order", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		var peerId = "p1"

		fx.setResolver(func(_ context.Context, _ string) ([]net.IPAddr, error) {
			return nil, fmt.Errorf("no such host")
		})
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"yamux://example.org:1111",
			"quic://example.org:1112",
		}, true)

		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "example.org:1112").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("servers never resolve: preferQuic unset short-circuits the local check", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.setResolver(func(_ context.Context, host string) ([]net.IPAddr, error) {
			t.Fatalf("resolver must not run with preferQuic=false, got lookup of %q", host)
			return nil, nil
		})
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"yamux://any-sync-node-1:1111",
			"quic://any-sync-node-1:1112",
		}, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "any-sync-node-1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("hostname verdict is cached", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		var peerId = "p1"
		var resolveCalls int

		fx.setResolver(func(_ context.Context, _ string) ([]net.IPAddr, error) {
			resolveCalls++
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.7")}}, nil
		})
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"yamux://box.lan:1111"}, true).Times(2)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "box.lan:1111").Return(fx.mockMC(peerId), nil).Times(2)

		for i := 0; i < 2; i++ {
			p, err := fx.Dial(ctx, peerId)
			require.NoError(t, err)
			assert.NotNil(t, p)
		}
		assert.Equal(t, 1, resolveCalls)
	})
}

func TestPeerService_DialWebTransport(t *testing.T) {
	t.Run("dial webtransport", func(t *testing.T) {
		fx := newFixtureWithWebTransport(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"
		var addrs = []string{"webtransport://127.0.0.1:4433"}

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.wt.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:4433").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("webtransport in preferred schemes", func(t *testing.T) {
		fx := newFixtureWithWebTransport(t)
		defer fx.finish(t)

		ps := fx.PeerService.(*peerService)
		schemes := ps.preferredSchemes(false)
		assert.Contains(t, schemes, transport.WebTransport)
	})
	t.Run("fallback to webtransport when yamux fails", func(t *testing.T) {
		fx := newFixtureWithWebTransport(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"
		var addrs = []string{
			"yamux://127.0.0.1:1111",
			"webtransport://127.0.0.1:4433",
		}

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").Return(nil, fmt.Errorf("yamux failed"))
		fx.wt.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:4433").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
}

func TestPeerService_Accept(t *testing.T) {
	fx := newFixture(t)
	defer fx.finish(t)

	mc := fx.mockMC("p1")
	require.NoError(t, fx.Accept(mc))
}

type fixture struct {
	PeerService
	a        *app.App
	ctrl     *gomock.Controller
	quic     mock_transport.TransportComponent
	yamux    mock_transport.TransportComponent
	nodeConf *mock_nodeconf.MockService
}

func newFixture(t *testing.T) *fixture {
	fx := newFixtureNoDemotion(t)
	fx.EnableQuicDemotion()
	return fx
}

func newFixtureNoDemotion(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	fx := &fixture{
		PeerService: New(),
		ctrl:        ctrl,
		a:           new(app.App),
		quic:        mock_transport.NewTransportComponent(ctrl, quic.CName),
		yamux:       mock_transport.NewTransportComponent(ctrl, yamux.CName),
		nodeConf:    mock_nodeconf.NewMockService(ctrl),
	}

	fx.quic.EXPECT().SetAccepter(fx.PeerService)
	fx.yamux.EXPECT().SetAccepter(fx.PeerService)

	fx.nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeConf.EXPECT().Init(gomock.Any())
	fx.nodeConf.EXPECT().Run(gomock.Any())
	fx.nodeConf.EXPECT().Close(gomock.Any())

	fx.a.Register(fx.PeerService).Register(fx.quic).Register(fx.yamux).Register(fx.nodeConf).Register(pool.New()).Register(rpctest.NewTestServer())

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func (fx *fixture) setResolver(resolve func(ctx context.Context, host string) ([]net.IPAddr, error)) {
	fx.PeerService.(*peerService).localAddrs.resolve = resolve
}

func (fx *fixture) mockMC(peerId string) *mock_transport.MockMultiConn {
	mc := mock_transport.NewMockMultiConn(fx.ctrl)
	cctx := peer.CtxWithPeerId(ctx, peerId)
	mc.EXPECT().Context().Return(cctx).AnyTimes()
	mc.EXPECT().Accept().Return(nil, fmt.Errorf("test")).AnyTimes()
	mc.EXPECT().Close().AnyTimes()
	// the pool subscribes to CloseChan to evict the peer when the connection
	// dies; keep the peer alive by returning a nil channel that never fires.
	mc.EXPECT().CloseChan().Return((<-chan struct{})(nil)).AnyTimes()
	return mc
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

type fixtureWithWT struct {
	*fixture
	wt mock_transport.TransportComponent
}

func newFixtureWithWebTransport(t *testing.T) *fixtureWithWT {
	ctrl := gomock.NewController(t)
	wt := mock_transport.NewTransportComponent(ctrl, webtransportpkg.CName)
	fx := &fixtureWithWT{
		fixture: &fixture{
			PeerService: New(),
			ctrl:        ctrl,
			a:           new(app.App),
			quic:        mock_transport.NewTransportComponent(ctrl, quic.CName),
			yamux:       mock_transport.NewTransportComponent(ctrl, yamux.CName),
			nodeConf:    mock_nodeconf.NewMockService(ctrl),
		},
		wt: wt,
	}

	fx.quic.EXPECT().SetAccepter(fx.PeerService)
	fx.yamux.EXPECT().SetAccepter(fx.PeerService)
	fx.wt.EXPECT().SetAccepter(fx.PeerService)

	fx.nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeConf.EXPECT().Init(gomock.Any())
	fx.nodeConf.EXPECT().Run(gomock.Any())
	fx.nodeConf.EXPECT().Close(gomock.Any())

	fx.a.Register(fx.PeerService).Register(fx.quic).Register(fx.yamux).Register(fx.wt).Register(fx.nodeConf).Register(pool.New()).Register(rpctest.NewTestServer())

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func TestPeerService_DialIroh(t *testing.T) {
	const peerId = "p1"
	const ticket = "endpointAAAAticket"
	t.Run("iroh addr is ignored without global dial ctx", func(t *testing.T) {
		fx := newFixtureWithIroh(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"iroh://" + ticket}, true)

		_, err := fx.Dial(ctx, peerId)
		assert.ErrorIs(t, err, ErrAddrsNotFound)
	})
	t.Run("iroh addr is dialed with global dial ctx", func(t *testing.T) {
		fx := newFixtureWithIroh(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"iroh://" + ticket}, true)
		fx.iroh.MockTransport.EXPECT().Dial(gomock.Any(), ticket).Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(CtxWithGlobalDial(ctx), peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("iroh ranks after lan addrs", func(t *testing.T) {
		fx := newFixtureWithIroh(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"iroh://" + ticket,
			"quic://1.2.3.4:1112",
			"yamux://1.2.3.4:1111",
		}, true)
		gomock.InOrder(
			fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "1.2.3.4:1112").Return(nil, fmt.Errorf("quic failed")),
			fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "1.2.3.4:1111").Return(nil, fmt.Errorf("yamux failed")),
			fx.iroh.MockTransport.EXPECT().Dial(gomock.Any(), ticket).Return(fx.mockMC(peerId), nil),
		)

		p, err := fx.Dial(CtxWithGlobalDial(ctx), peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("iroh addr never hits the local resolver", func(t *testing.T) {
		fx := newFixtureWithIroh(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		fx.setResolver(func(ctx context.Context, host string) ([]net.IPAddr, error) {
			t.Fatalf("resolver called for %q", host)
			return nil, nil
		})

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"iroh://" + ticket}, true)
		fx.iroh.MockTransport.EXPECT().Dial(gomock.Any(), ticket).Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(CtxWithGlobalDial(ctx), peerId)
		require.NoError(t, err)
	})
	t.Run("iroh is the last preferred scheme", func(t *testing.T) {
		fx := newFixtureWithIroh(t)
		defer fx.finish(t)

		ps := fx.PeerService.(*peerService)
		for _, preferQuic := range []bool{false, true} {
			schemes := ps.preferredSchemes(preferQuic)
			assert.Equal(t, transport.Iroh, schemes[len(schemes)-1])
		}
	})
	t.Run("global dial ctx flag", func(t *testing.T) {
		assert.False(t, ctxIsGlobalDial(ctx))
		assert.True(t, ctxIsGlobalDial(CtxWithGlobalDial(ctx)))
	})
	t.Run("tickets are shortened in logs", func(t *testing.T) {
		assert.Equal(t, "iroh://endpointabcd…", logAddr("iroh://endpointabcdefghijklmnop"))
		assert.Equal(t, "yamux://1.2.3.4:1", logAddr("yamux://1.2.3.4:1"))
	})
}

type fixtureWithIroh struct {
	*fixture
	iroh mock_transport.TransportComponent
}

func newFixtureWithIroh(t *testing.T) *fixtureWithIroh {
	ctrl := gomock.NewController(t)
	fx := &fixtureWithIroh{
		fixture: &fixture{
			PeerService: New(),
			ctrl:        ctrl,
			a:           new(app.App),
			quic:        mock_transport.NewTransportComponent(ctrl, quic.CName),
			yamux:       mock_transport.NewTransportComponent(ctrl, yamux.CName),
			nodeConf:    mock_nodeconf.NewMockService(ctrl),
		},
		iroh: mock_transport.NewTransportComponent(ctrl, transport.IrohCName),
	}

	fx.quic.EXPECT().SetAccepter(fx.PeerService)
	fx.yamux.EXPECT().SetAccepter(fx.PeerService)
	fx.iroh.EXPECT().SetAccepter(fx.PeerService)

	fx.nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeConf.EXPECT().Init(gomock.Any())
	fx.nodeConf.EXPECT().Run(gomock.Any())
	fx.nodeConf.EXPECT().Close(gomock.Any())

	fx.a.Register(fx.PeerService).Register(fx.quic).Register(fx.yamux).Register(fx.iroh).Register(fx.nodeConf).Register(pool.New()).Register(rpctest.NewTestServer())

	require.NoError(t, fx.a.Start(ctx))
	return fx
}
