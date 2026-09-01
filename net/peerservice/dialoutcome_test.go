package peerservice

import (
	"context"
	"fmt"
	"testing"

	quicgo "github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/quicdemotion"
	"github.com/anyproto/any-sync/net/transport"
)

// What the dial loop learns and hands to the demotion component, and whether
// its verdict reorders the schemes. Everything the component then decides -
// strikes, demotion, probes - belongs to net/quicdemotion and is tested there.

// stubDemotion records what the dial loop reports and answers the reorder
// question on demand, so these tests never depend on the demotion policy.
type stubDemotion struct {
	demote   bool
	outcomes []quicdemotion.DialOutcome
}

func (s *stubDemotion) Init(a *app.App) error           { return nil }
func (s *stubDemotion) Name() string                    { return quicdemotion.CName }
func (s *stubDemotion) Run(ctx context.Context) error   { return nil }
func (s *stubDemotion) Close(ctx context.Context) error { return nil }

func (s *stubDemotion) DemoteDial(peerId string) bool          { return s.demote }
func (s *stubDemotion) ObserveDial(o quicdemotion.DialOutcome) { s.outcomes = append(s.outcomes, o) }
func (s *stubDemotion) Snapshot() quicdemotion.PenaltySnapshot { return quicdemotion.PenaltySnapshot{} }
func (s *stubDemotion) Seed(snap quicdemotion.PenaltySnapshot) {}
func (s *stubDemotion) Reset()                                 {}
func (s *stubDemotion) SetObserver(observer func())            {}

func (s *stubDemotion) only(t *testing.T) quicdemotion.DialOutcome {
	t.Helper()
	require.Len(t, s.outcomes, 1, "exactly one outcome per Dial")
	return s.outcomes[0]
}

func newFixtureWithStubDemotion(t *testing.T) (*fixture, *stubDemotion) {
	stub := &stubDemotion{}
	fx := newFixtureNoDemotion(t, stub)
	fx.PreferQuic(true)
	return fx, stub
}

var demotionAddrs = []string{
	"yamux://203.0.113.1:1111",
	"quic://203.0.113.1:1112",
}

func TestPeerService_DemotionReordersSchemes(t *testing.T) {
	const peerId = "p1"

	t.Run("a demoted peer is dialed yamux-first", func(t *testing.T) {
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)
		stub.demote = true

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("an undemoted peer keeps the quic preference", func(t *testing.T) {
		fx, _ := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(fx.mockMC(peerId), nil)

		p, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
	t.Run("without the component registered nothing is demoted", func(t *testing.T) {
		// a server node: the component is simply not registered, so dialing
		// behaves exactly as it did before the feature existed
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

func TestPeerService_DialOutcome(t *testing.T) {
	const peerId = "p1"

	// A blackholed UDP path makes quic-go give up with an idle timeout, not a
	// handshake timeout, so this is the shape the dial loop must recognise.
	t.Run("quic timed out and yamux carried the dial", func(t *testing.T) {
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.Equal(t, quicdemotion.DialOutcome{
			PeerId:          peerId,
			QuicTimedOut:    true,
			SucceededScheme: transport.Yamux,
		}, stub.only(t))
	})
	t.Run("several quic addresses timing out are still one outcome", func(t *testing.T) {
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"yamux://203.0.113.1:1111",
			"quic://203.0.113.1:1112",
			"quic://203.0.113.1:1113",
		}, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1113").Return(nil, &quicgo.HandshakeTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.Equal(t, transport.Yamux, stub.only(t).SucceededScheme)
	})
	t.Run("nothing connected", func(t *testing.T) {
		// scheme("") returns yamux, so an outcome derived from the connected
		// address would invert this and report the fallback as working
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(nil, fmt.Errorf("network is unreachable"))

		_, err := fx.Dial(ctx, peerId)
		require.Error(t, err)
		assert.Equal(t, quicdemotion.DialOutcome{
			PeerId:         peerId,
			QuicTimedOut:   true,
			FallbackFailed: true,
		}, stub.only(t))
	})
	t.Run("quic itself carried the dial", func(t *testing.T) {
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"quic://203.0.113.1:1112",
			"quic://203.0.113.1:1113",
		}, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1113").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.Equal(t, transport.Quic, stub.only(t).SucceededScheme)
	})
	t.Run("an ordinary quic error is not a timeout", func(t *testing.T) {
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, fmt.Errorf("connection refused"))
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.False(t, stub.only(t).QuicTimedOut)
	})
	t.Run("a failing quic dial is not a failing fallback", func(t *testing.T) {
		// only yamux says anything about the fallback
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{"quic://203.0.113.1:1112"}, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, fmt.Errorf("connection refused"))

		_, err := fx.Dial(ctx, peerId)
		require.Error(t, err)
		assert.False(t, stub.only(t).FallbackFailed)
	})
	t.Run("a connection that is opened and then rejected reached nobody", func(t *testing.T) {
		fx, stub := newFixtureWithStubDemotion(t)
		defer fx.finish(t)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return(demotionAddrs, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.yamux.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1111").Return(fx.mockMC("someoneElse"), nil)

		_, err := fx.Dial(ctx, peerId)
		require.ErrorIs(t, err, ErrPeerIdMismatched)
		assert.Empty(t, stub.only(t).SucceededScheme, "a rejected connection is not a working fallback")
	})
	t.Run("webtransport is not a fallback", func(t *testing.T) {
		// webtransport runs over quic, so succeeding on it proves udp works
		fx := newFixtureWithWebTransport(t)
		defer fx.finish(t)
		fx.PreferQuic(true)
		stub := fx.demotion.(*stubDemotion)

		fx.nodeConf.EXPECT().PeerAddresses(peerId).Return([]string{
			"quic://203.0.113.1:1112",
			"webtransport://203.0.113.1:4433",
		}, true)
		fx.quic.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:1112").Return(nil, &quicgo.IdleTimeoutError{})
		fx.wt.MockTransport.EXPECT().Dial(gomock.Any(), "203.0.113.1:4433").Return(fx.mockMC(peerId), nil)

		_, err := fx.Dial(ctx, peerId)
		require.NoError(t, err)
		assert.Equal(t, transport.WebTransport, stub.only(t).SucceededScheme,
			"reported as-is; that webtransport proves udp works is the component's business")
	})
}
