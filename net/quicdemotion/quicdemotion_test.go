package quicdemotion

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
)

var ctx = context.Background()

// stubObservableTransport stands in for the quic transport so the test can
// see whether the component subscribed to connection close events.
type stubObservableTransport struct {
	observer func(ev transport.ConnCloseEvent)
}

func (s *stubObservableTransport) Init(a *app.App) error                   { return nil }
func (s *stubObservableTransport) Name() string                            { return quic.CName }
func (s *stubObservableTransport) SetAccepter(accepter transport.Accepter) {}
func (s *stubObservableTransport) Dial(ctx context.Context, addr string) (transport.MultiConn, error) {
	return nil, nil
}
func (s *stubObservableTransport) SetConnObserver(observer func(ev transport.ConnCloseEvent)) {
	s.observer = observer
}

type fixture struct {
	Service
	a        *app.App
	ctrl     *gomock.Controller
	quic     *stubObservableTransport
	nodeConf *mock_nodeconf.MockService
	nodeIds  map[string]bool
}

func newFixture(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	fx := &fixture{
		Service:  New(),
		a:        new(app.App),
		ctrl:     ctrl,
		quic:     &stubObservableTransport{},
		nodeConf: mock_nodeconf.NewMockService(ctrl),
		nodeIds:  map[string]bool{},
	}
	fx.nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeConf.EXPECT().Init(gomock.Any())
	fx.nodeConf.EXPECT().Run(gomock.Any())
	fx.nodeConf.EXPECT().Close(gomock.Any())
	fx.nodeConf.EXPECT().NodeTypes(gomock.Any()).DoAndReturn(func(id string) []nodeconf.NodeType {
		if fx.nodeIds[id] {
			return []nodeconf.NodeType{nodeconf.NodeTypeTree}
		}
		return nil
	}).AnyTimes()

	fx.a.Register(fx.Service).Register(fx.quic).Register(fx.nodeConf)
	require.NoError(t, fx.a.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, fx.a.Close(ctx))
		ctrl.Finish()
	})
	return fx
}

func (fx *fixture) svc() *service { return fx.Service.(*service) }

func TestService_SubscribesToConnCloses(t *testing.T) {
	// registering the component is what turns the feature on: a server node
	// leaves it out and the transport reports to nobody
	fx := newFixture(t)
	assert.NotNil(t, fx.quic.observer, "the component must subscribe to quic conn closes during Init")
}

func TestService_OnConnClosed(t *testing.T) {
	const peerId = "p1"
	t.Run("degraded deaths demote the peer", func(t *testing.T) {
		fx := newFixture(t)
		for i := 0; i < demoteThreshold; i++ {
			fx.svc().onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
		}
		assert.True(t, fx.DemoteDial(peerId))
	})
	t.Run("a healthy death clears the strikes", func(t *testing.T) {
		fx := newFixture(t)
		fx.svc().onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
		fx.svc().onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseHealthy})
		assert.Empty(t, fx.Snapshot().Peers)
	})
	t.Run("neutral deaths are not strikes", func(t *testing.T) {
		// the common case in production - pool eviction, shutdown, a graceful
		// remote close. If it counted, two ordinary disconnects would demote
		// a peer and no quic connection would ever form to clear it again.
		fx := newFixture(t)
		for i := 0; i < demoteThreshold+2; i++ {
			fx.svc().onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseNeutral})
		}
		assert.Empty(t, fx.Snapshot().Peers)
		assert.False(t, fx.DemoteDial(peerId))
	})
	t.Run("an event without a peer id is ignored", func(t *testing.T) {
		fx := newFixture(t)
		fx.svc().onConnClosed(transport.ConnCloseEvent{Kind: transport.ConnCloseDegraded})
		assert.Empty(t, fx.Snapshot().Peers)
	})
}

func TestService_ObserveDial(t *testing.T) {
	const peerId = "p1"
	t.Run("a quic timeout counts when another transport carried the dial", func(t *testing.T) {
		fx := newFixture(t)
		fx.ObserveDial(DialOutcome{PeerId: peerId, QuicTimedOut: true, SucceededScheme: transport.Yamux})
		assert.Equal(t, 1, fx.Snapshot().Peers[peerId].ConsecutiveDegraded)
	})
	t.Run("a quic timeout is no evidence when everything failed", func(t *testing.T) {
		fx := newFixture(t)
		fx.ObserveDial(DialOutcome{PeerId: peerId, QuicTimedOut: true, FallbackFailed: true})
		assert.Empty(t, fx.Snapshot().Peers)
	})
	t.Run("a quic timeout is no evidence when quic itself then worked", func(t *testing.T) {
		fx := newFixture(t)
		fx.ObserveDial(DialOutcome{PeerId: peerId, QuicTimedOut: true, SucceededScheme: transport.Quic})
		assert.Empty(t, fx.Snapshot().Peers)
	})
	t.Run("a failing fallback suspends demotion", func(t *testing.T) {
		fx := newFixture(t)
		fx.nodeIds["other"] = true
		for i := 0; i < demoteThreshold; i++ {
			fx.svc().onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
		}
		require.True(t, fx.DemoteDial(peerId))

		fx.ObserveDial(DialOutcome{PeerId: "other", FallbackFailed: true})
		assert.False(t, fx.DemoteDial(peerId), "yamux is failing too, so preferring it helps nobody")

		fx.ObserveDial(DialOutcome{PeerId: "other", SucceededScheme: transport.Yamux})
		assert.True(t, fx.DemoteDial(peerId))
	})
	t.Run("a failing dial to a non-node peer does not suspend demotion", func(t *testing.T) {
		// LAN peers are dialed constantly and go to sleep; their failures say
		// nothing about whether tcp works toward the network
		fx := newFixture(t)
		for i := 0; i < demoteThreshold; i++ {
			fx.svc().onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
		}
		require.True(t, fx.DemoteDial(peerId))

		fx.ObserveDial(DialOutcome{PeerId: "sleepingPhone", FallbackFailed: true})
		assert.True(t, fx.DemoteDial(peerId))
	})
}

func TestService_PersistenceSurface(t *testing.T) {
	const peerId = "p1"
	fx := newFixture(t)
	var observed int
	fx.SetObserver(func() { observed++ })

	for i := 0; i < demoteThreshold; i++ {
		fx.svc().onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
	}
	assert.Equal(t, demoteThreshold, observed, "each mutation must reach the client that persists it")

	snap := fx.Snapshot()
	require.Contains(t, snap.Peers, peerId)

	fx.Reset()
	assert.Empty(t, fx.Snapshot().Peers)

	fx.Seed(snap)
	assert.True(t, fx.DemoteDial(peerId), "a restored verdict applies straight away")
}

func TestService_WithoutQuicTransport(t *testing.T) {
	// heart can be configured yamux-only; the component must still start
	ctrl := gomock.NewController(t)
	nodeConf := mock_nodeconf.NewMockService(ctrl)
	nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	nodeConf.EXPECT().Init(gomock.Any())
	nodeConf.EXPECT().Run(gomock.Any())
	nodeConf.EXPECT().Close(gomock.Any())

	a := new(app.App)
	svc := New()
	a.Register(svc).Register(nodeConf)
	require.NoError(t, a.Start(ctx))
	defer func() { require.NoError(t, a.Close(ctx)) }()

	assert.False(t, svc.DemoteDial("p1"))
}
