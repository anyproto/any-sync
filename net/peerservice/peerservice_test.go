package peerservice

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

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

func TestPeerService_DialParallel(t *testing.T) {
	// public (non-local) addrs so per-address local ordering stays out of
	// the picture; PreferQuic(false) ⇒ yamux is the preferred candidate.
	var addrs = []string{
		"yamux://203.0.113.1:1111",
		"quic://203.0.113.1:1112",
	}
	setStagger := func(t *testing.T, d time.Duration) {
		prev := dialStaggerInterval
		dialStaggerInterval = d
		t.Cleanup(func() { dialStaggerInterval = prev })
	}
	t.Run("disabled by default: no second dial while the first hangs", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		setStagger(t, 20*time.Millisecond)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		// Only yamux is expected: sequential mode must not touch the quic
		// candidate while the first attempt is still in flight.
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").DoAndReturn(
			func(ctx context.Context, addr string) (transport.MultiConn, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			})

		dialCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		p, err := fx.Dial(dialCtx, peerId)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("blackholed addr does not serialize the dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		fx.SetParallelDial(true)
		setStagger(t, 30*time.Millisecond)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		// The preferred candidate hangs like a filtered port: nothing
		// answers until the dial ctx is cancelled.
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").DoAndReturn(
			func(ctx context.Context, addr string) (transport.MultiConn, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			})
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

		start := time.Now()
		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Less(t, time.Since(start), 2*time.Second,
			"the second candidate must win after one stagger interval, not after the first dial's timeout")
	})
	t.Run("all candidates fail returns joined errors", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		fx.SetParallelDial(true)
		setStagger(t, 30*time.Millisecond)
		var peerId = "p1"
		errYamux := fmt.Errorf("yamux down")
		errQuic := fmt.Errorf("quic down")

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, errYamux)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, errQuic)

		p, err := fx.Dial(ctx, peerId)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, errYamux)
		assert.ErrorIs(t, err, errQuic)
	})
	t.Run("late success is closed", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		fx.SetParallelDial(true)
		setStagger(t, 20*time.Millisecond)
		var peerId = "p1"

		closed := make(chan struct{})
		late := mock_transport.NewMockMultiConn(fx.ctrl)
		late.EXPECT().Close().DoAndReturn(func() error {
			close(closed)
			return nil
		})

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		// The preferred candidate succeeds — but only after the race is
		// already decided; its connection must be closed by the drain.
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").DoAndReturn(
			func(ctx context.Context, addr string) (transport.MultiConn, error) {
				// Wide margin over the stagger so the quic candidate wins
				// even on a heavily loaded CI scheduler.
				time.Sleep(500 * time.Millisecond)
				return late, nil
			})
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		select {
		case <-closed:
		case <-time.After(2 * time.Second):
			t.Fatal("late-winning connection was never closed")
		}
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
