package pool

import (
	"context"
	"errors"
	"fmt"
	net2 "net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
)

var ctx = context.Background()

func TestPool_Get(t *testing.T) {
	t.Run("dial error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		var expErr = errors.New("dial error")
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return nil, expErr
		}
		p, err := fx.Get(ctx, "1")
		assert.Nil(t, p)
		assert.EqualError(t, err, expErr.Error())
	})
	t.Run("dial and cached", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return newTestPeer("1", time.Now(), 0, 0), nil
		}
		p, err := fx.Get(ctx, "1")
		assert.NoError(t, err)
		assert.NotNil(t, p)
		fx.Dialer.dial = nil
		p, err = fx.Get(ctx, "1")
		assert.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("retry for closed", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		tp := newTestPeer("1", time.Now(), 0, 0)
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return tp, nil
		}
		p, err := fx.Get(ctx, "1")
		assert.NoError(t, err)
		assert.NotNil(t, p)
		p.Close()
		tp2 := newTestPeer("1", time.Now(), 0, 0)
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return tp2, nil
		}
		p, err = fx.Get(ctx, "1")
		assert.NoError(t, err)
		assert.Equal(t, p, tp2)
	})
}

func TestPool_GetOneOf(t *testing.T) {
	addToCache := func(t *testing.T, fx *fixture, tp *testPeer) {
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return tp, nil
		}
		gp, err := fx.Get(ctx, tp.Id())
		require.NoError(t, err)
		require.Equal(t, gp, tp)
	}

	t.Run("from cache", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		tp1 := newTestPeer("1", time.Now(), 0, 0)
		addToCache(t, fx, tp1)
		p, err := fx.GetOneOf(ctx, []string{"3", "2", "1"})
		require.NoError(t, err)
		assert.Equal(t, tp1, p)
	})
	t.Run("from cache - skip closed", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		tp2 := newTestPeer("2", time.Now(), 0, 0)
		addToCache(t, fx, tp2)
		tp2.Close()
		tp1 := newTestPeer("1", time.Now(), 0, 0)
		addToCache(t, fx, tp1)
		p, err := fx.GetOneOf(ctx, []string{"3", "2", "1"})
		require.NoError(t, err)
		assert.Equal(t, tp1, p)
	})
	t.Run("dial", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		var called bool
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			if called {
				return nil, fmt.Errorf("not expected call")
			}
			called = true
			return newTestPeer(peerId, time.Now(), 0, 0), nil
		}
		p, err := fx.GetOneOf(ctx, []string{"3", "2", "1"})
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("unable to connect", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return nil, fmt.Errorf("persistent error")
		}
		p, err := fx.GetOneOf(ctx, []string{"3", "2", "1"})
		assert.Equal(t, net.ErrUnableToConnect, err)
		assert.Nil(t, p)
	})
	t.Run("handshake error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return nil, handshake.ErrIncompatibleVersion
		}
		p, err := fx.GetOneOf(ctx, []string{"3", "2", "1"})
		assert.Equal(t, handshake.ErrIncompatibleVersion, err)
		assert.Nil(t, p)
	})
}

func TestPool_AddPeer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		require.NoError(t, fx.AddPeer(ctx, newTestPeer("p1", time.Now(), 0, 0)))
	})
	t.Run("two peers", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p1, p2 := newTestPeer("p1", time.Now(), 0, 0), newTestPeer("p1", time.Now(), 0, 0)
		require.NoError(t, fx.AddPeer(ctx, p1))
		require.NoError(t, fx.AddPeer(ctx, p2))
		select {
		case <-p1.closed:
		default:
			assert.Truef(t, false, "peer not closed")
		}
	})
}

func TestPool_Pick(t *testing.T) {
	t.Run("not exist", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()

		p, err := fx.Pick(ctx, "1")
		assert.Nil(t, p)
		assert.NotNil(t, err)
	})
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p1 := newTestPeer("p1", time.Now(), 0, 0)
		require.NoError(t, fx.AddPeer(ctx, p1))

		p, err := fx.Pick(ctx, "p1")

		assert.NotNil(t, p)
		assert.Equal(t, p1, p)
		assert.Nil(t, err)
	})
	t.Run("peer is closed", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish()
		p1 := newTestPeer("p1", time.Now(), 0, 0)
		require.NoError(t, fx.AddPeer(ctx, p1))
		require.NoError(t, p1.Close())
		p, err := fx.Pick(ctx, "p1")

		assert.Nil(t, p)
		assert.NotNil(t, err)
	})
}

func TestProvideStat_NoPeers(t *testing.T) {
	t.Run("only incoming peers", func(t *testing.T) {
		// given
		fx := newFixture(t)
		defer fx.Finish()
		created := time.Now()
		p1 := newTestPeer("p1", created, 1, 1)
		require.NoError(t, fx.AddPeer(ctx, p1))

		statProvider, ok := fx.Service.(*poolService)
		assert.True(t, ok)

		// when
		stat := statProvider.ProvideStat()

		// then
		assert.NotNil(t, stat)
		poolStat, ok := stat.(*poolStats)
		assert.True(t, ok)

		assert.Len(t, poolStat.PeerStats, 1)
		assert.Equal(t, p1.id, poolStat.PeerStats[0].PeerId)
		assert.Equal(t, p1.created, poolStat.PeerStats[0].Created)
		assert.Equal(t, p1.subConnections, poolStat.PeerStats[0].SubConnections)
		assert.Equal(t, p1.version, poolStat.PeerStats[0].Version)
	})
	t.Run("outgoing and incoming peers", func(t *testing.T) {
		// given
		fx := newFixture(t)
		defer fx.Finish()
		created := time.Now()
		created1 := time.Now()
		p1 := newTestPeer("p1", created, 2, 2)
		require.NoError(t, fx.AddPeer(ctx, p1))

		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return newTestPeer(peerId, created1, 0, 0), nil
		}

		peerId := "p2"
		_, err := fx.Get(ctx, peerId)
		require.NoError(t, err)

		statProvider, ok := fx.Service.(*poolService)
		assert.True(t, ok)

		// when
		stat := statProvider.ProvideStat()

		// then
		assert.NotNil(t, stat)
		poolStat, ok := stat.(*poolStats)
		assert.True(t, ok)

		assert.Len(t, poolStat.PeerStats, 2)
		assert.Equal(t, peerId, poolStat.PeerStats[0].PeerId)
		assert.Equal(t, created1, poolStat.PeerStats[0].Created)
		assert.Equal(t, 0, poolStat.PeerStats[0].SubConnections)
		assert.Equal(t, uint32(0), poolStat.PeerStats[0].Version)

		assert.Equal(t, p1.id, poolStat.PeerStats[1].PeerId)
		assert.Equal(t, p1.created, poolStat.PeerStats[1].Created)
		assert.Equal(t, p1.subConnections, poolStat.PeerStats[1].SubConnections)
		assert.Equal(t, p1.version, poolStat.PeerStats[1].Version)
	})
	t.Run("only outcoming peers", func(t *testing.T) {
		// given
		peerId := "p1"
		fx := newFixture(t)
		defer fx.Finish()
		created := time.Now()
		subConn := 3
		version := uint32(2)

		fx.Dialer.dial = func(ctx context.Context, peerId string) (peer peer.Peer, err error) {
			return newTestPeer(peerId, created, subConn, version), nil
		}

		_, err := fx.Get(ctx, peerId)
		require.NoError(t, err)

		statProvider, ok := fx.Service.(*poolService)
		assert.True(t, ok)

		// when
		stat := statProvider.ProvideStat()

		// then
		assert.NotNil(t, stat)
		poolStat, ok := stat.(*poolStats)
		assert.True(t, ok)

		assert.Len(t, poolStat.PeerStats, 1)
		assert.Equal(t, peerId, poolStat.PeerStats[0].PeerId)
		assert.Equal(t, created, poolStat.PeerStats[0].Created)
		assert.Equal(t, subConn, poolStat.PeerStats[0].SubConnections)
		assert.Equal(t, version, poolStat.PeerStats[0].Version)
	})
	t.Run("no peers", func(t *testing.T) {
		// given
		fx := newFixture(t)
		defer fx.Finish()

		statProvider, ok := fx.Service.(*poolService)
		assert.True(t, ok)

		// when
		stat := statProvider.ProvideStat()

		// then
		assert.NotNil(t, stat)
		poolStat, ok := stat.(*poolStats)
		assert.True(t, ok)
		assert.Len(t, poolStat.PeerStats, 0)
	})
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		Service: New(),
		Dialer:  &dialerMock{},
	}
	a := new(app.App)
	a.Register(fx.Service)
	a.Register(fx.Dialer)
	require.NoError(t, a.Start(context.Background()))
	fx.a = a
	fx.t = t
	return fx
}

func (fx *fixture) Finish() {
	require.NoError(fx.t, fx.a.Close(context.Background()))
}

type fixture struct {
	Service
	Dialer *dialerMock
	a      *app.App
	t      *testing.T
}

var _ dialer = (*dialerMock)(nil)

type dialerMock struct {
	dial func(ctx context.Context, peerId string) (peer peer.Peer, err error)
}

func (d *dialerMock) Dial(ctx context.Context, peerId string) (peer peer.Peer, err error) {
	return d.dial(ctx, peerId)
}

func (d *dialerMock) UpdateAddrs(addrs map[string][]string) {
	return
}

func (d *dialerMock) SetPeerAddrs(peerId string, addrs []string) {
	return
}

func (d *dialerMock) Init(a *app.App) (err error) {
	return
}

func (d *dialerMock) Name() (name string) {
	return "net.peerservice"
}

func newTestPeer(id string, created time.Time, subConnections int, version uint32) *testPeer {
	return &testPeer{
		id:             id,
		closed:         make(chan struct{}),
		created:        created,
		subConnections: subConnections,
		version:        version,
	}
}

type testPeer struct {
	id             string
	closed         chan struct{}
	created        time.Time
	subConnections int
	version        uint32
}

func (t *testPeer) ProvideStat() *peer.Stat {
	return &peer.Stat{
		PeerId:         t.id,
		Created:        t.created,
		SubConnections: t.subConnections,
		Version:        t.version,
	}
}

func (t *testPeer) SetTTL(ttl time.Duration) {
	return
}

func (t *testPeer) DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error {
	return fmt.Errorf("not implemented")
}

func (t *testPeer) AcquireDrpcConn(ctx context.Context) (drpc.Conn, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *testPeer) ReleaseDrpcConn(conn drpc.Conn) {}

func (t *testPeer) Context() context.Context {
	//TODO implement me
	panic("implement me")
}

func (t *testPeer) Accept() (conn net2.Conn, err error) {
	//TODO implement me
	panic("implement me")
}

func (t *testPeer) Open(ctx context.Context) (conn net2.Conn, err error) {
	//TODO implement me
	panic("implement me")
}

func (t *testPeer) Addr() string {
	return ""
}

func (t *testPeer) Id() string {
	return t.id
}

func (t *testPeer) TryClose(objectTTL time.Duration) (res bool, err error) {
	return true, t.Close()
}

func (t *testPeer) Close() error {
	select {
	case <-t.closed:
		return fmt.Errorf("already closed")
	default:
		close(t.closed)
	}
	return nil
}

func (t *testPeer) IsClosed() bool {
	select {
	case <-t.closed:
		return true
	default:
		return false
	}
}

func (t *testPeer) CloseChan() <-chan struct{} {
	return t.closed
}
