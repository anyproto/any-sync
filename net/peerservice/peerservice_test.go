package peerservice

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/peerobserver"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/quicdemotion"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
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

func TestPeerService_PeerObserver(t *testing.T) {
	// public (non-local) addrs: the global preferQuic order applies
	var addrs = []string{
		"yamux://203.0.113.1:1111",
		"quic://203.0.113.1:1112",
	}
	t.Run("successful dial reports started then connected, in order", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindConnected}, kindsOf(events))
		assert.Equal(t, "p1", events[0].PeerId)
		assert.Equal(t, 2, events[0].AddrCount)
		connected := events[1]
		assert.Equal(t, "p1", connected.PeerId)
		assert.Equal(t, "203.0.113.1:1111", connected.Addr)
		assert.Equal(t, transport.Yamux, connected.Scheme)
		assert.False(t, connected.Inbound)
		assert.Equal(t, uint32(13), connected.ProtoVersion)
		assert.Greater(t, connected.Dur, time.Duration(0))
	})
	t.Run("failed dial reports started then failed with every address error", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		errYamuxRefused := errors.New("yamux refused")
		errQuicTimeout := errors.New("quic timed out")
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, errYamuxRefused)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, errQuicTimeout)

		_, err := fx.Dial(ctx, peerId)
		require.Error(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindDialFailed}, kindsOf(events))
		failed := events[1]
		assert.Equal(t, "p1", failed.PeerId)
		// errors.Is must see through the join: consumers classify by sentinel
		assert.ErrorIs(t, failed.Err, errYamuxRefused)
		assert.ErrorIs(t, failed.Err, errQuicTimeout)
		assert.Greater(t, failed.Dur, time.Duration(0))
	})
	t.Run("a single failing address still arrives joined", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		errRefused := errors.New("yamux refused")
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"yamux://203.0.113.1:1111"}, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, errRefused)

		_, err := fx.Dial(ctx, peerId)
		require.Error(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindDialFailed}, kindsOf(events))
		assert.ErrorIs(t, events[1].Err, errRefused)
		// the promised stable shape: joined even for one address
		_, joined := events[1].Err.(interface{ Unwrap() []error })
		assert.True(t, joined, "a single address error must still arrive joined")
	})
	t.Run("addr count reflects dialable candidates, not raw addrs", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		// webtransport has no registered transport in this fixture, so only
		// the yamux addr is a dial candidate
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"yamux://203.0.113.1:1111",
			"webtransport://203.0.113.1:4433",
		}, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindConnected}, kindsOf(events))
		assert.Equal(t, 1, events[0].AddrCount)
	})
	t.Run("no addrs reports failed with ErrAddrsNotFound", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(nil, false)

		_, err := fx.Dial(ctx, peerId)
		require.Error(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindDialFailed}, kindsOf(events))
		assert.Equal(t, 0, events[0].AddrCount)
		assert.ErrorIs(t, events[1].Err, ErrAddrsNotFound)
	})
	t.Run("mismatched peerId reports failed", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId+"not valid"), nil)

		_, err := fx.Dial(ctx, peerId)
		require.Error(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindDialFailed}, kindsOf(events))
		assert.ErrorIs(t, events[1].Err, ErrPeerIdMismatched)
	})
	t.Run("accept reports inbound connected with scheme from its address", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)

		mc := fx.mockMC("p1")
		require.NoError(t, fx.Accept(mc))

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindConnected}, kindsOf(events))
		connected := events[0]
		assert.Equal(t, "p1", connected.PeerId)
		assert.Equal(t, "192.0.2.7:3333", connected.Addr)
		assert.Equal(t, transport.Yamux, connected.Scheme)
		assert.True(t, connected.Inbound)
		assert.Equal(t, uint32(13), connected.ProtoVersion)
		assert.Zero(t, connected.Dur)
	})
	t.Run("proto version missing from conn context reports zero", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)

		mc := fx.mockMCWithCtxAddr(peer.CtxWithPeerId(ctx, "p1"), "192.0.2.7:3333")
		require.NoError(t, fx.Accept(mc))

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindConnected}, kindsOf(events))
		assert.Zero(t, events[0].ProtoVersion)
		assert.Empty(t, events[0].Scheme)
	})
	t.Run("inbound connection that never becomes a peer produces no event", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)

		// no peer id in the conn context: peer.NewPeer must fail
		mc := fx.mockMCWithCtxAddr(ctx, "yamux://192.0.2.7:3333")
		require.Error(t, fx.Accept(mc))
		assert.Empty(t, obs.getEvents())
	})
	t.Run("connected is emitted before the pool learns of the peer", func(t *testing.T) {
		// deterministic pin of the ordering comment in Accept: at AddPeer
		// entry the Connected event must already have been delivered
		obs := &peerEventRecorder{}
		ocp := &orderCheckPool{obs: obs}
		fx := newFixtureCustom(t, ocp, peerobserver.New(obs), quicdemotion.New())
		defer fx.finish(t)

		mc := fx.mockMC("p1")
		require.NoError(t, fx.Accept(mc))
		require.Equal(t, []peerobserver.Kind{peerobserver.KindConnected}, ocp.kindsOnAdd)
	})
	t.Run("pool call for another peer from a dial-path event is safe", func(t *testing.T) {
		obs := &crossPeerObserver{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		obs.pool = fx.a.MustComponent(pool.CName).(pool.Service)

		fx.nodeConf.EXPECT().PeerAddresses("p1").Return([]string{"yamux://203.0.113.1:1111"}, true)
		fx.nodeConf.EXPECT().PeerAddresses("p2").Return([]string{"yamux://203.0.113.1:2222"}, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC("p1"), nil)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:2222").Return(fx.mockMC("p2"), nil)

		_, err := obs.pool.Get(ctx, "p1")
		require.NoError(t, err)
		require.True(t, obs.getDone())
		require.NoError(t, obs.getOtherErr())
	})
	t.Run("accept into refusing pool reports connected then closed", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithRefusingPool(t, obs)
		defer fx.finish(t)

		mc := fx.mockMC("p1")
		require.Error(t, fx.Accept(mc))

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindConnected, peerobserver.KindClosed}, kindsOf(events))
		assert.True(t, events[1].Inbound)
		assert.Equal(t, "p1", events[1].PeerId)
	})
	t.Run("concurrent pool gets share one dial and one event pair", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true).AnyTimes()
		release := make(chan struct{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").DoAndReturn(
			func(ctx context.Context, addr string) (transport.MultiConn, error) {
				<-release
				return fx.mockMC(peerId), nil
			})

		pl := fx.a.MustComponent(pool.CName).(pool.Service)
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = pl.Get(ctx, peerId)
			}()
		}
		close(release)
		wg.Wait()

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindConnected}, kindsOf(events))
	})
	t.Run("cached incompatible-version verdict produces no further events", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"yamux://203.0.113.1:1111"}, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, handshake.ErrIncompatibleVersion)

		pl := fx.a.MustComponent(pool.CName).(pool.Service)
		_, err := pl.Get(ctx, peerId)
		require.Error(t, err)
		_, err = pl.Get(ctx, peerId)
		require.Error(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindDialFailed}, kindsOf(events))
	})
	t.Run("dial that falls back to the second addr reports started then connected only", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, fmt.Errorf("yamux refused"))
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindConnected}, kindsOf(events))
		assert.Equal(t, transport.Quic, events[1].Scheme)
		assert.Equal(t, "203.0.113.1:1112", events[1].Addr)
	})
	t.Run("pool call for the dialed peer from a dial-path event blocks until ctx dies", func(t *testing.T) {
		// pins the documented hazard: dial-path events run inside the pool's
		// single-flight load, so a pool call for the same peer cannot proceed
		obs := &reentrantObserver{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"
		obs.pool = fx.a.MustComponent(pool.CName).(pool.Service)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		_, err := obs.pool.Get(ctx, peerId)
		require.NoError(t, err)
		require.ErrorIs(t, obs.getSamePeerErr(), context.DeadlineExceeded)
	})
	t.Run("accepted connection that dies reports connected then closed, in order", func(t *testing.T) {
		// composed peerservice+pool path: the pool watcher's Closed must
		// follow the Connected emitted by Accept
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)

		closedCh := make(chan struct{})
		close(closedCh)
		cctx := peer.CtxWithProtoVersion(peer.CtxWithPeerId(ctx, "p1"), 13)
		mc := mock_transport.NewMockMultiConn(fx.ctrl)
		mc.EXPECT().Context().Return(cctx).AnyTimes()
		mc.EXPECT().Addr().Return("yamux://192.0.2.7:3333").AnyTimes()
		mc.EXPECT().IsClosed().Return(true).AnyTimes()
		mc.EXPECT().CloseChan().Return((<-chan struct{})(closedCh)).AnyTimes()
		mc.EXPECT().Close().Return(nil).AnyTimes()
		mc.EXPECT().Accept().Return(nil, fmt.Errorf("test")).AnyTimes()

		require.NoError(t, fx.Accept(mc))

		require.Eventually(t, func() bool { return len(obs.getEvents()) == 2 }, time.Second, 10*time.Millisecond)
		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindConnected, peerobserver.KindClosed}, kindsOf(events))
		assert.True(t, events[1].Inbound)
	})
	t.Run("iroh accept reports node id and iroh scheme", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithObserver(t, obs)
		defer fx.finish(t)

		cctx := peer.CtxWithProtoVersion(peer.CtxWithPeerId(ctx, "p1"), 13)
		mc := fx.mockMCWithCtxAddr(cctx, "iroh://k51qzi5uqu5dgutdk6i1")
		require.NoError(t, fx.Accept(mc))

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindConnected}, kindsOf(events))
		assert.Equal(t, transport.Iroh, events[0].Scheme)
		assert.Equal(t, "k51qzi5uqu5dgutdk6i1", events[0].Addr)
		assert.True(t, events[0].Inbound)
	})
	t.Run("outbound iroh dial reports a shortened ticket", func(t *testing.T) {
		// an iroh ticket encodes the peer's relay and IP addresses; the
		// observer must receive it shortened, the way logs shorten it
		obs := &peerEventRecorder{}
		fx := newFixtureWithIrohObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"
		ticket := "abcdefghijklmnopqrstuvwxyz0123456789"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"iroh://" + ticket}, true)
		fx.iroh.MockTransport.EXPECT().Dial(gomock.Any(), ticket).Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(CtxWithGlobalDial(ctx), peerId)
		require.NoError(t, err)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindConnected}, kindsOf(events))
		assert.Equal(t, 1, events[0].AddrCount)
		connected := events[1]
		assert.Equal(t, transport.Iroh, connected.Scheme)
		assert.Equal(t, "abcdefghijkl…", connected.Addr)
		assert.NotContains(t, connected.Addr, ticket[12:], "the full ticket must not reach the observer")
	})
	t.Run("iroh addrs are not dial candidates without the global-dial ctx", func(t *testing.T) {
		obs := &peerEventRecorder{}
		fx := newFixtureWithIrohObserver(t, obs)
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"iroh://someticket"}, true)

		_, err := fx.Dial(ctx, peerId)
		require.ErrorIs(t, err, ErrAddrsNotFound)

		events := obs.getEvents()
		require.Equal(t, []peerobserver.Kind{peerobserver.KindDialStarted, peerobserver.KindDialFailed}, kindsOf(events))
		assert.Equal(t, 0, events[0].AddrCount)
		assert.ErrorIs(t, events[1].Err, ErrAddrsNotFound)
	})
	t.Run("panicking observer does not break dialing or accepting", func(t *testing.T) {
		fx := newFixtureWithObserver(t, panickyPeerObserver{})
		defer fx.finish(t)
		fx.PreferQuic(false)
		var peerId = "p1"

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(addrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		require.NoError(t, fx.Accept(fx.mockMC("p2")))
	})
}

type peerEventRecorder struct {
	mu     sync.Mutex
	events []peerobserver.Event
}

func (r *peerEventRecorder) ObservePeerEvent(ev peerobserver.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *peerEventRecorder) getEvents() []peerobserver.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]peerobserver.Event(nil), r.events...)
}

func kindsOf(events []peerobserver.Event) []peerobserver.Kind {
	kinds := make([]peerobserver.Kind, 0, len(events))
	for _, ev := range events {
		kinds = append(kinds, ev.Kind)
	}
	return kinds
}

// reentrantObserver calls back into the pool for the peer a Connected event
// names, with a short deadline, recording the resulting error
type reentrantObserver struct {
	pool        pool.Service
	mu          sync.Mutex
	samePeerErr error
}

func (r *reentrantObserver) ObservePeerEvent(ev peerobserver.Event) {
	if ev.Kind != peerobserver.KindConnected {
		return
	}
	pctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := r.pool.Get(pctx, ev.PeerId)
	r.mu.Lock()
	r.samePeerErr = err
	r.mu.Unlock()
}

func (r *reentrantObserver) getSamePeerErr() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.samePeerErr
}

type panickyPeerObserver struct{}

func (panickyPeerObserver) ObservePeerEvent(peerobserver.Event) { panic("observer panic") }

// crossPeerObserver calls the pool for a DIFFERENT peer from inside a
// dial-path Connected event — allowed by the contract. The guard is a CAS,
// not sync.Once: the nested dial's own Connected re-enters this method on
// the same goroutine, and a re-entrant once.Do would self-block
type crossPeerObserver struct {
	pool     pool.Service
	started  atomic.Bool
	mu       sync.Mutex
	otherErr error
	done     bool
}

func (o *crossPeerObserver) ObservePeerEvent(ev peerobserver.Event) {
	if ev.Kind != peerobserver.KindConnected {
		return
	}
	if !o.started.CompareAndSwap(false, true) {
		return
	}
	pctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := o.pool.Get(pctx, "p2")
	o.mu.Lock()
	o.otherErr = err
	o.done = true
	o.mu.Unlock()
}

func (o *crossPeerObserver) getDone() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.done
}

func (o *crossPeerObserver) getOtherErr() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.otherErr
}

// orderCheckPool satisfies pool.Service (methods beyond the overridden ones
// panic on the nil embedded interface — sufficient because Accept only calls
// AddPeer) and records which events the observer had already received when
// AddPeer was entered
type orderCheckPool struct {
	pool.Service
	obs        *peerEventRecorder
	kindsOnAdd []peerobserver.Kind
}

func (p *orderCheckPool) Init(a *app.App) error           { return nil }
func (p *orderCheckPool) Name() string                    { return pool.CName }
func (p *orderCheckPool) Run(ctx context.Context) error   { return nil }
func (p *orderCheckPool) Close(ctx context.Context) error { return nil }
func (p *orderCheckPool) AddPeer(ctx context.Context, pr peer.Peer) error {
	p.kindsOnAdd = kindsOf(p.obs.getEvents())
	return nil
}

// refusingPool satisfies pool.Service but rejects every AddPeer, to reach
// Accept's failure branch; methods beyond the overridden ones panic on the
// nil embedded interface — sufficient because Accept only calls AddPeer
type refusingPool struct {
	pool.Service
}

func (r *refusingPool) Init(a *app.App) error           { return nil }
func (r *refusingPool) Name() string                    { return pool.CName }
func (r *refusingPool) Run(ctx context.Context) error   { return nil }
func (r *refusingPool) Close(ctx context.Context) error { return nil }
func (r *refusingPool) AddPeer(ctx context.Context, p peer.Peer) error {
	return fmt.Errorf("pool refuses")
}

type fixture struct {
	PeerService
	a        *app.App
	ctrl     *gomock.Controller
	quic     mock_transport.TransportComponent
	yamux    mock_transport.TransportComponent
	nodeConf *mock_nodeconf.MockService
	// nodeIds marks peer ids the nodeconf reports as network nodes
	nodeIds map[string]bool
	// demotion is the optional quic demotion component, nil when not registered
	demotion quicdemotion.Service
}

func newFixture(t *testing.T) *fixture {
	return newFixtureNoDemotion(t, quicdemotion.New())
}

func newFixtureWithObserver(t *testing.T, obs peerobserver.Observer) *fixture {
	return newFixtureNoDemotion(t, peerobserver.New(obs), quicdemotion.New())
}

func newFixtureWithIrohObserver(t *testing.T, obs peerobserver.Observer) *fixtureWithIroh {
	ctrl := gomock.NewController(t)
	fx := &fixtureWithIroh{
		fixture: &fixture{
			PeerService: New(),
			ctrl:        ctrl,
			a:           new(app.App),
			quic:        mock_transport.NewTransportComponent(ctrl, quic.CName),
			yamux:       mock_transport.NewTransportComponent(ctrl, yamux.CName),
			nodeConf:    mock_nodeconf.NewMockService(ctrl),
			nodeIds:     map[string]bool{},
		},
		iroh: mock_transport.NewTransportComponent(ctrl, transport.IrohCName),
	}
	fx.demotion = quicdemotion.New()

	fx.quic.EXPECT().SetAccepter(fx.PeerService)
	fx.yamux.EXPECT().SetAccepter(fx.PeerService)
	fx.iroh.EXPECT().SetAccepter(fx.PeerService)

	fx.nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeConf.EXPECT().Init(gomock.Any())
	fx.nodeConf.EXPECT().Run(gomock.Any())
	fx.nodeConf.EXPECT().Close(gomock.Any())

	fx.a.Register(fx.PeerService).Register(fx.quic).Register(fx.yamux).Register(fx.iroh).Register(fx.nodeConf).Register(pool.New()).Register(rpctest.NewTestServer()).Register(peerobserver.New(obs)).Register(fx.demotion)

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func newFixtureWithRefusingPool(t *testing.T, obs peerobserver.Observer) *fixture {
	return newFixtureCustom(t, &refusingPool{}, peerobserver.New(obs), quicdemotion.New())
}

func newFixtureNoDemotion(t *testing.T, extra ...app.Component) *fixture {
	return newFixtureCustom(t, pool.New(), extra...)
}

func newFixtureCustom(t *testing.T, poolComponent app.Component, extra ...app.Component) *fixture {
	ctrl := gomock.NewController(t)
	fx := &fixture{
		PeerService: New(),
		ctrl:        ctrl,
		a:           new(app.App),
		quic:        mock_transport.NewTransportComponent(ctrl, quic.CName),
		yamux:       mock_transport.NewTransportComponent(ctrl, yamux.CName),
		nodeConf:    mock_nodeconf.NewMockService(ctrl),
		nodeIds:     map[string]bool{},
	}
	for _, comp := range extra {
		if demotion, ok := comp.(quicdemotion.Service); ok {
			fx.demotion = demotion
		}
	}

	fx.quic.EXPECT().SetAccepter(fx.PeerService)
	fx.yamux.EXPECT().SetAccepter(fx.PeerService)

	fx.nodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.nodeConf.EXPECT().NodeTypes(gomock.Any()).DoAndReturn(func(id string) []nodeconf.NodeType {
		if fx.nodeIds[id] {
			return []nodeconf.NodeType{nodeconf.NodeTypeTree}
		}
		return nil
	}).AnyTimes()
	fx.nodeConf.EXPECT().Init(gomock.Any())
	fx.nodeConf.EXPECT().Run(gomock.Any())
	fx.nodeConf.EXPECT().Close(gomock.Any())

	fx.a.Register(fx.PeerService).Register(fx.quic).Register(fx.yamux).Register(fx.nodeConf).Register(poolComponent).Register(rpctest.NewTestServer())
	for _, comp := range extra {
		fx.a.Register(comp)
	}

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func (fx *fixture) setResolver(resolve func(ctx context.Context, host string) ([]net.IPAddr, error)) {
	fx.PeerService.(*peerService).localAddrs.resolve = resolve
}

func (fx *fixture) mockMC(peerId string) *mock_transport.MockMultiConn {
	cctx := peer.CtxWithProtoVersion(peer.CtxWithPeerId(ctx, peerId), 13)
	// real transports return scheme-prefixed addresses
	return fx.mockMCWithCtxAddr(cctx, "yamux://192.0.2.7:3333")
}

func (fx *fixture) mockMCWithCtxAddr(cctx context.Context, addr string) *mock_transport.MockMultiConn {
	mc := mock_transport.NewMockMultiConn(fx.ctrl)
	mc.EXPECT().Context().Return(cctx).AnyTimes()
	mc.EXPECT().Addr().Return(addr).AnyTimes()
	mc.EXPECT().IsClosed().Return(false).AnyTimes()
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
