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
