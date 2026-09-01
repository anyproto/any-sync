package peerservice

import (
	"context"
	"fmt"
	"testing"
	"time"

	quicgo "github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/mock_transport"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
)

type penaltyFixture struct {
	*transportPenalties
	clock    time.Time
	nodeIds  map[string]bool
	observed int
}

func newPenaltyFixture() *penaltyFixture {
	fx := &penaltyFixture{
		clock:   time.Unix(1000000, 0),
		nodeIds: map[string]bool{},
	}
	fx.transportPenalties = newTransportPenalties(
		func() time.Time { return fx.clock },
		func(peerId string) bool { return fx.nodeIds[peerId] },
	)
	fx.enable()
	fx.setObserver(func() { fx.observed++ })
	return fx
}

func (fx *penaltyFixture) advance(d time.Duration) {
	fx.clock = fx.clock.Add(d)
}

func TestTransportPenalties_Demote(t *testing.T) {
	t.Run("single degraded event does not demote", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		assert.False(t, fx.quicDemoted("p1"))
	})
	t.Run("two consecutive degraded events demote", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		assert.True(t, fx.quicDemoted("p1"))
		assert.False(t, fx.quicDemoted("p2"))
	})
	t.Run("healthy outcome clears the state", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		fx.registerHealthy("p1")
		assert.False(t, fx.quicDemoted("p1"))
		// and the strike memory is gone: one death doesn't re-demote
		fx.registerDegraded("p1")
		assert.False(t, fx.quicDemoted("p1"))
	})
}

func TestTransportPenalties_TTL(t *testing.T) {
	t.Run("demotion expires after base TTL", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		fx.advance(demotionBaseTTL + time.Second)
		assert.False(t, fx.quicDemoted("p1"))
	})
	t.Run("one degraded death after expiry re-demotes with doubled TTL", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		fx.advance(demotionBaseTTL + time.Second)
		fx.registerDegraded("p1")
		assert.True(t, fx.quicDemoted("p1"))
		// still demoted just before the doubled TTL runs out
		fx.advance(2*demotionBaseTTL - time.Second)
		assert.True(t, fx.quicDemoted("p1"))
		fx.advance(2 * time.Second)
		assert.False(t, fx.quicDemoted("p1"))
	})
	t.Run("TTL is capped", func(t *testing.T) {
		fx := newPenaltyFixture()
		for i := 0; i < 20; i++ {
			fx.registerDegraded("p1")
		}
		fx.advance(demotionMaxTTL + time.Second)
		assert.False(t, fx.quicDemoted("p1"))
	})
}

func TestTransportPenalties_GlobalDemotion(t *testing.T) {
	t.Run("two demoted network nodes demote every peer", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.nodeIds["n1"] = true
		fx.nodeIds["n2"] = true
		for _, id := range []string{"n1", "n2"} {
			fx.registerDegraded(id)
			fx.registerDegraded(id)
		}
		assert.True(t, fx.quicDemoted("someOtherPeer"))
	})
	t.Run("demoted p2p peers do not count toward global demotion", func(t *testing.T) {
		fx := newPenaltyFixture()
		for _, id := range []string{"phone1", "phone2"} {
			fx.registerDegraded(id)
			fx.registerDegraded(id)
		}
		assert.True(t, fx.quicDemoted("phone1"))
		assert.False(t, fx.quicDemoted("someOtherPeer"))
	})
	t.Run("global demotion ends when a node demotion expires", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.nodeIds["n1"] = true
		fx.nodeIds["n2"] = true
		fx.registerDegraded("n1")
		fx.registerDegraded("n1")
		fx.advance(time.Minute)
		fx.registerDegraded("n2")
		fx.registerDegraded("n2")
		assert.True(t, fx.quicDemoted("someOtherPeer"))
		// n1 expires first, dropping the demoted-node count below the bar
		fx.advance(demotionBaseTTL - time.Minute + time.Second)
		assert.False(t, fx.quicDemoted("someOtherPeer"))
		assert.True(t, fx.quicDemoted("n2"))
	})
}

func TestTransportPenalties_SnapshotSeedReset(t *testing.T) {
	t.Run("snapshot and seed round-trip", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		fx.registerDegraded("p2")

		snap := fx.snapshot()
		require.Len(t, snap.Peers, 2)

		fx2 := newPenaltyFixture()
		fx2.clock = fx.clock
		fx2.seed(snap)
		assert.True(t, fx2.quicDemoted("p1"))
		assert.False(t, fx2.quicDemoted("p2"))
		// seeded strike memory works: one more death demotes p2
		fx2.registerDegraded("p2")
		assert.True(t, fx2.quicDemoted("p2"))
	})
	t.Run("reset clears everything", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		fx.reset()
		assert.False(t, fx.quicDemoted("p1"))
		assert.Empty(t, fx.snapshot().Peers)
	})
}

func TestPeerService_QuicDemotion(t *testing.T) {
	var addrs = []string{
		"yamux://203.0.113.1:1111",
		"quic://203.0.113.1:1112",
	}
	var peerId = "p1"

	demotedSnapshot := func(id string) PenaltySnapshot {
		return PenaltySnapshot{Peers: map[string]PeerPenalty{
			id: {
				ConsecutiveDegraded: demoteThreshold - 1,
				DemotedUntil:        time.Now().Add(time.Hour),
				BackoffLevel:        1,
			},
		}}
	}

	t.Run("demoted peer is dialed yamux-first", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		fx.SeedTransportPenalties(demotedSnapshot(peerId))

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("reset restores quic preference", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		fx.SeedTransportPenalties(demotedSnapshot(peerId))
		fx.ResetTransportPenalties()

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("degraded conn deaths demote the peer", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		ps := fx.PeerService.(*peerService)

		for i := 0; i < demoteThreshold; i++ {
			ps.onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
		}

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("healthy conn death clears the strikes", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		ps := fx.PeerService.(*peerService)

		ps.onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
		ps.onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseHealthy})

		assert.Empty(t, fx.TransportPenalties().Peers)
	})
	t.Run("quic handshake timeout counts one strike per dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		// two quic addrs: both time out within one Dial, still a single strike
		var multiQuicAddrs = []string{
			"yamux://203.0.113.1:1111",
			"quic://203.0.113.1:1112",
			"quic://203.0.113.1:1113",
		}

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(multiQuicAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.HandshakeTimeoutError{})
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1113").Return(nil, &quicgo.HandshakeTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, 1, fx.TransportPenalties().Peers[peerId].ConsecutiveDegraded)
	})
	t.Run("non-timeout quic dial errors are not strikes", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, fmt.Errorf("connection refused"))
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Empty(t, fx.TransportPenalties().Peers)
	})
	t.Run("two demoted network nodes demote everyone", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		ps := fx.PeerService.(*peerService)

		fx.nodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()
		for _, id := range []string{"n1", "n2"} {
			for i := 0; i < demoteThreshold; i++ {
				ps.onConnClosed(transport.ConnCloseEvent{PeerId: id, Kind: transport.ConnCloseDegraded})
			}
		}

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("penalty observer fires on mutations", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		ps := fx.PeerService.(*peerService)

		var fired int
		fx.SetPenaltyObserver(func() { fired++ })
		ps.onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
		assert.Equal(t, 1, fired)
	})
}

// stubObservableTransport implements transport.Transport plus
// transport.ConnObserverSetter to check that peerservice registers its conn
// observer on the quic transport during Init.
type stubObservableTransport struct {
	name     string
	observer func(ev transport.ConnCloseEvent)
}

func (s *stubObservableTransport) Init(a *app.App) error                   { return nil }
func (s *stubObservableTransport) Name() string                            { return s.name }
func (s *stubObservableTransport) SetAccepter(accepter transport.Accepter) {}
func (s *stubObservableTransport) Dial(ctx context.Context, addr string) (transport.MultiConn, error) {
	return nil, fmt.Errorf("stub")
}
func (s *stubObservableTransport) SetConnObserver(observer func(ev transport.ConnCloseEvent)) {
	s.observer = observer
}

func TestPeerService_RegistersConnObserver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	fx := &fixture{
		PeerService: New(),
		ctrl:        ctrl,
		a:           new(app.App),
		yamux:       mock_transport.NewTransportComponent(ctrl, yamux.CName),
		nodeConf:    mock_nodeconf.NewMockService(ctrl),
	}
	stub := &stubObservableTransport{name: quic.CName}

	fx.yamux.EXPECT().SetAccepter(fx.PeerService)
	fx.nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeConf.EXPECT().Init(gomock.Any())
	fx.nodeConf.EXPECT().Run(gomock.Any())
	fx.nodeConf.EXPECT().Close(gomock.Any())

	fx.a.Register(fx.PeerService).Register(stub).Register(fx.yamux).Register(fx.nodeConf).Register(pool.New()).Register(rpctest.NewTestServer())
	require.NoError(t, fx.a.Start(ctx))
	defer fx.finish(t)

	assert.Nil(t, stub.observer, "demotion is opt-in: no observer before EnableQuicDemotion")
	fx.EnableQuicDemotion()
	assert.NotNil(t, stub.observer, "EnableQuicDemotion must register a conn observer on the quic transport")
}

func TestPeerService_DemotionDisabledByDefault(t *testing.T) {
	// without EnableQuicDemotion degraded events must not change dial order
	var addrs = []string{
		"yamux://203.0.113.1:1111",
		"quic://203.0.113.1:1112",
	}
	fx := newFixtureNoDemotion(t)
	defer fx.finish(t)
	fx.PreferQuic(true)
	var peerId = "p1"
	ps := fx.PeerService.(*peerService)

	for i := 0; i < demoteThreshold; i++ {
		ps.onConnClosed(transport.ConnCloseEvent{PeerId: peerId, Kind: transport.ConnCloseDegraded})
	}
	assert.Empty(t, fx.TransportPenalties().Peers)

	fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
	fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

	p, err := fx.Dial(ctx, peerId)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestTransportPenalties_Observer(t *testing.T) {
	t.Run("observer fires on state mutations", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		assert.Equal(t, 1, fx.observed)
		fx.registerDegraded("p1")
		assert.Equal(t, 2, fx.observed)
		fx.registerHealthy("p1")
		assert.Equal(t, 3, fx.observed)
	})
	t.Run("observer does not fire without a mutation", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerHealthy("unknownPeer")
		fx.reset()
		assert.Equal(t, 0, fx.observed)
	})
}
