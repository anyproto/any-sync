package peerservice

import (
	"fmt"
	"testing"
	"time"

	quicgo "github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/net/quicdemotion"
)

// These cover the seam between dialing and the demotion component: what the
// dial loop reports to it, and how its verdict reorders the schemes. The
// policy behind the verdict is tested in net/quicdemotion.

var demotionAddrs = []string{
	"yamux://203.0.113.1:1111",
	"quic://203.0.113.1:1112",
}

func demotedSnapshot(peerId string) quicdemotion.PenaltySnapshot {
	return quicdemotion.PenaltySnapshot{
		Version: quicdemotion.PenaltySnapshotVersion,
		Peers: map[string]quicdemotion.PeerPenalty{
			peerId: {
				DemotedUntil: time.Now().Add(time.Hour),
				NextProbeAt:  time.Now().Add(time.Hour),
			},
		},
	}
}

func TestPeerService_QuicDemotion(t *testing.T) {
	const peerId = "p1"

	t.Run("a demoted peer is dialed yamux-first", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		fx.demotion.Seed(demotedSnapshot(peerId))

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("reset restores the quic preference", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		fx.demotion.Seed(demotedSnapshot(peerId))
		fx.demotion.Reset()

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("without the component registered nothing is demoted", func(t *testing.T) {
		// a server node: the demotion component is simply not registered, so
		// dialing behaves exactly as it did before the feature existed
		fx := newFixtureNoDemotion(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
}

func TestPeerService_DialOutcomeReporting(t *testing.T) {
	const peerId = "p1"

	// A blackholed UDP path makes quic-go give up with an idle timeout, not a
	// handshake timeout, so this is the shape the dial loop must recognise.
	t.Run("a quic timeout with a working fallback is reported as degraded", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, 1, fx.demotion.Snapshot().Peers[peerId].ConsecutiveDegraded)
	})
	t.Run("one strike per dial however many quic addrs time out", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"yamux://203.0.113.1:1111",
			"quic://203.0.113.1:1112",
			"quic://203.0.113.1:1113",
		}, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1113").Return(nil, &quicgo.HandshakeTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, 1, fx.demotion.Snapshot().Peers[peerId].ConsecutiveDegraded)
	})
	t.Run("nothing is reported when every transport fails", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, fmt.Errorf("network is unreachable"))

		_, err := fx.Dial(ctx, peerId)
		require.Error(t, err)
		assert.Empty(t, fx.demotion.Snapshot().Peers)
	})
	t.Run("nothing is reported when quic itself then works", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"quic://203.0.113.1:1112",
			"quic://203.0.113.1:1113",
		}, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1113").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Empty(t, fx.demotion.Snapshot().Peers)
	})
	t.Run("an ordinary quic error is not a timeout", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, fmt.Errorf("connection refused"))
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Empty(t, fx.demotion.Snapshot().Peers)
	})
	t.Run("a failing yamux dial suspends demotion", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		fx.nodeIds[peerId] = true
		fx.demotion.Seed(demotedSnapshot(peerId))

		// yamux is tried first because of the demotion, fails, and quic carries
		// the dial - so the fallback is now known to be broken
		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true).Times(2)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, fmt.Errorf("connection refused"))
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil).Times(2)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)

		// the next dial goes quic-first: demoting onto a dead fallback is pointless
		_, err = fx.Dial(ctx, peerId)
		require.NoError(t, err)
	})
}

func TestPeerService_TotalDialFailureIsNotAFallbackSuccess(t *testing.T) {
	// A dial where nothing connected must be reported as the fallback
	// failing, never as yamux succeeding - note scheme("") returns yamux, so
	// deriving the winning scheme from an empty address silently inverts this.
	fx := newFixture(t)
	defer fx.finish(t)
	fx.PreferQuic(true)
	fx.nodeIds["dead"], fx.nodeIds["demoted"] = true, true
	fx.demotion.Seed(demotedSnapshot("demoted"))

	fx.nodeConf.EXPECT().PeerAddresses("dead").Return(demotionAddrs, true)
	fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
	fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, fmt.Errorf("network is unreachable"))

	_, err := fx.Dial(ctx, "dead")
	require.Error(t, err)

	// yamux is known broken now, so the demoted peer must be dialed quic-first
	fx.nodeConf.EXPECT().PeerAddresses("demoted").Return(demotionAddrs, true)
	fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC("demoted"), nil)

	p, err := fx.Dial(ctx, "demoted")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestPeerService_RejectedConnectionIsNotASuccess(t *testing.T) {
	// A dial that connects and is then rejected (wrong peer behind a stale
	// address) reached nobody: it must not count as the fallback working, nor
	// attribute a quic timeout to a peer we never actually talked to.
	const peerId = "p1"
	fx := newFixture(t)
	defer fx.finish(t)
	fx.PreferQuic(true)
	fx.nodeIds[peerId] = true

	fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
	fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
	fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC("someoneElse"), nil)

	_, err := fx.Dial(ctx, peerId)
	require.ErrorIs(t, err, ErrPeerIdMismatched)
	assert.Empty(t, fx.demotion.Snapshot().Peers, "a rejected connection is not evidence about the path")
}

func TestPeerService_WebtransportIsNotAFallback(t *testing.T) {
	// webtransport runs over quic, so succeeding on it proves UDP works -
	// the opposite of what a strike would record.
	const peerId = "p1"
	fx := newFixtureWithWebTransport(t)
	defer fx.finish(t)
	fx.PreferQuic(true)

	fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
		"quic://203.0.113.1:1112",
		"webtransport://203.0.113.1:4433",
	}, true)
	fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
	fx.wt.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:4433").Return(fx.mockMC(peerId), nil)

	p, err := fx.Dial(ctx, peerId)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Empty(t, fx.demotion.Snapshot().Peers, "udp demonstrably works, so nothing was learned against quic")
}

func TestPeerService_QuicFailureIsNotAFallbackFailure(t *testing.T) {
	// Only a yamux dial says anything about the fallback. A quic-only peer
	// whose dial fails must not be read as "tcp is broken", which would
	// suspend demotion everywhere.
	const peerId = "p1"
	fx := newFixture(t)
	defer fx.finish(t)
	fx.PreferQuic(true)
	fx.nodeIds[peerId], fx.nodeIds["demoted"] = true, true
	fx.demotion.Seed(demotedSnapshot("demoted"))

	fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"quic://203.0.113.1:1112"}, true)
	fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, fmt.Errorf("connection refused"))
	_, err := fx.Dial(ctx, peerId)
	require.Error(t, err)

	// the demoted peer must still be dialed yamux-first
	fx.nodeConf.EXPECT().PeerAddresses("demoted").Return(demotionAddrs, true)
	fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC("demoted"), nil)
	p, err := fx.Dial(ctx, "demoted")
	require.NoError(t, err)
	assert.NotNil(t, p)
}
