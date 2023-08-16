package peerservice

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/transport/mock_transport"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
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

		fx.yamux.MockTransport.EXPECT().Dial(ctx, "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

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

		fx.quic.MockTransport.EXPECT().Dial(ctx, "127.0.0.1:1112").Return(fx.mockMC(peerId), nil)

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

		fx.quic.MockTransport.EXPECT().Dial(ctx, "127.0.0.1:1112").Return(nil, fmt.Errorf("test"))
		fx.yamux.MockTransport.EXPECT().Dial(ctx, "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

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

		fx.yamux.MockTransport.EXPECT().Dial(ctx, "127.0.0.1:1111").Return(fx.mockMC(peerId+"not valid"), nil)

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

		fx.yamux.MockTransport.EXPECT().Dial(ctx, "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

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

		fx.yamux.MockTransport.EXPECT().Dial(ctx, "127.0.0.1:1111").Return(fx.mockMC(peerId), nil)

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
