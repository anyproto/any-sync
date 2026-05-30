package peerservice

import (
	"context"
	"fmt"
	"testing"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/secureservice"
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
	var addrs = []string{
		"yamux://127.0.0.1:1111",
		"quic://127.0.0.1:1112",
	}
	t.Run("prefer yamux", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

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

		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1112").Return(fx.mockMC(peerId), nil)

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

		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1112").Return(nil, fmt.Errorf("test"))
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

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

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").Return(fx.mockMC(peerId+"not valid"), nil)

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

		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

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
		schemes := ps.preferredSchemes()
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

func TestPeerService_DialAdmissionTokenProvider(t *testing.T) {
	const peerId = "p1"
	addrs := []string{"yamux://127.0.0.1:1111"}

	t.Run("injects provider token", func(t *testing.T) {
		provider := &testAdmissionTokenProvider{token: "provider-token"}
		fx := newFixtureWithAdmissionTokenProvider(t, provider)
		defer fx.finish(t)
		fx.PreferQuic(false)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").DoAndReturn(
			func(dialCtx context.Context, addr string) (transport.MultiConn, error) {
				assert.Equal(t, "provider-token", secureservice.CtxOutboundAdmissionToken(dialCtx))
				expectedPeerId, err := peer.CtxExpectedPeerId(dialCtx)
				require.NoError(t, err)
				assert.Equal(t, peerId, expectedPeerId)
				return fx.mockMC(peerId), nil
			})

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		require.Len(t, provider.requests, 1)
		assert.Equal(t, peerId, provider.requests[0].peerId)
	})

	t.Run("preserves existing token", func(t *testing.T) {
		provider := &testAdmissionTokenProvider{token: "provider-token"}
		fx := newFixtureWithAdmissionTokenProvider(t, provider)
		defer fx.finish(t)
		fx.PreferQuic(false)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "127.0.0.1:1111").DoAndReturn(
			func(dialCtx context.Context, addr string) (transport.MultiConn, error) {
				assert.Equal(t, "caller-token", secureservice.CtxOutboundAdmissionToken(dialCtx))
				return fx.mockMC(peerId), nil
			})

		p, err := fx.Dial(secureservice.CtxWithOutboundAdmissionToken(ctx, "caller-token"), peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Empty(t, provider.requests)
	})

	t.Run("provider error fails dial", func(t *testing.T) {
		providerErr := fmt.Errorf("provider failed")
		provider := &testAdmissionTokenProvider{err: providerErr}
		fx := newFixtureWithAdmissionTokenProvider(t, provider)
		defer fx.finish(t)
		fx.PreferQuic(false)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)

		p, err := fx.Dial(ctx, peerId)
		assert.Equal(t, providerErr, err)
		assert.Nil(t, p)
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
	return newFixtureWithAdmissionTokenProvider(t, nil)
}

func newFixtureWithAdmissionTokenProvider(t *testing.T, admissionTokenProvider *testAdmissionTokenProvider) *fixture {
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

	if admissionTokenProvider != nil {
		fx.a.Register(admissionTokenProvider)
	}
	fx.a.Register(fx.PeerService).Register(fx.quic).Register(fx.yamux).Register(fx.nodeConf).Register(pool.New()).Register(rpctest.NewTestServer())

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type testAdmissionTokenProvider struct {
	token    string
	err      error
	requests []admissionTokenRequest
}

type admissionTokenRequest struct {
	ctx    context.Context
	peerId string
}

func (p *testAdmissionTokenProvider) Init(a *app.App) error { return nil }

func (p *testAdmissionTokenProvider) Name() string { return "test.admissionTokenProvider" }

func (p *testAdmissionTokenProvider) OutboundAdmissionToken(ctx context.Context, peerId string) (string, error) {
	p.requests = append(p.requests, admissionTokenRequest{ctx: ctx, peerId: peerId})
	return p.token, p.err
}

func (fx *fixture) mockMC(peerId string) *mock_transport.MockMultiConn {
	mc := mock_transport.NewMockMultiConn(fx.ctrl)
	cctx := peer.CtxWithPeerId(ctx, peerId)
	mc.EXPECT().Context().Return(cctx).AnyTimes()
	mc.EXPECT().Accept().Return(nil, fmt.Errorf("test")).AnyTimes()
	mc.EXPECT().Close().AnyTimes()
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
