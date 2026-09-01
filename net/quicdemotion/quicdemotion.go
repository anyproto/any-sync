// Package quicdemotion watches quic connections for the signature of a path
// that swallows packets - a middlebox that lets the handshake through and
// then drops the flow - and tells the peer service to dial such peers
// yamux-first until the path recovers.
//
// It is an optional component: register it and the peer service picks it up,
// leave it out and dialing behaves exactly as it did before. Server nodes
// leave it out.
package quicdemotion

import (
	"context"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
)

const CName = "net.quicdemotion"

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

// DialOutcome summarises one Dial: which schemes were tried, whether quic
// timed out the way a blocked path does, and what ended up carrying the dial.
type DialOutcome struct {
	PeerId string
	// QuicTimedOut is set when a quic address gave up with a timeout that
	// means the path swallowed our packets.
	QuicTimedOut bool
	// SucceededScheme is the scheme that finally connected, empty if the dial
	// failed entirely.
	SucceededScheme string
	// FallbackFailed is set when a yamux address was tried and failed.
	FallbackFailed bool
}

type Service interface {
	app.ComponentRunnable
	// DemoteDial reports whether this dial should put yamux ahead of quic.
	DemoteDial(peerId string) bool
	// ObserveDial records what one completed Dial learned about the peer.
	ObserveDial(outcome DialOutcome)
	// Snapshot returns a copy of the penalty state for persistence.
	Snapshot() PenaltySnapshot
	// Seed restores a previously stored snapshot.
	Seed(snap PenaltySnapshot)
	// Reset drops all penalty state, e.g. after a network change: a new
	// network deserves a clean verdict.
	Reset()
	// SetObserver registers a callback fired after every state mutation, so a
	// client can persist the snapshot. Must not block.
	SetObserver(observer func())
}

type service struct {
	penalties *transportPenalties
	nodeConf  nodeconf.NodeConf
	quic      transport.Transport
}

func (s *service) Init(a *app.App) error {
	s.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	s.penalties = newTransportPenalties(time.Now, func(peerId string) bool {
		return len(s.nodeConf.NodeTypes(peerId)) > 0
	})
	if comp := a.Component(quic.CName); comp != nil {
		s.quic = comp.(transport.Transport)
		if setter, ok := s.quic.(transport.ConnObserverSetter); ok {
			setter.SetConnObserver(s.onConnClosed)
		}
	}
	return nil
}

func (s *service) Name() string { return CName }

func (s *service) Run(ctx context.Context) error { return nil }

func (s *service) Close(ctx context.Context) error { return nil }

// onConnClosed receives close events of dialed quic connections: degraded
// deaths accumulate strikes toward demoting the peer, a healthy one clears
// them.
func (s *service) onConnClosed(ev transport.ConnCloseEvent) {
	if ev.PeerId == "" {
		return
	}
	switch ev.Kind {
	case transport.ConnCloseDegraded:
		demoted := s.penalties.registerDegraded(ev.PeerId)
		log.Info("quic conn died degraded",
			zap.String("peerId", ev.PeerId),
			zap.Duration("lifetime", ev.Lifetime),
			zap.Int64("bytesRead", ev.BytesRead),
			zap.Int64("bytesWritten", ev.BytesWritten),
			zap.Bool("demoted", demoted),
			zap.Error(ev.Cause))
	case transport.ConnCloseHealthy:
		s.penalties.registerHealthy(ev.PeerId)
	}
}

func (s *service) DemoteDial(peerId string) bool {
	return s.penalties.demoteDial(peerId)
}

func (s *service) ObserveDial(outcome DialOutcome) {
	if outcome.SucceededScheme == transport.Yamux {
		s.penalties.recordFallback(outcome.PeerId, true)
	} else if outcome.FallbackFailed {
		s.penalties.recordFallback(outcome.PeerId, false)
	}
	// A timed-out quic dial only says something about UDP if another
	// transport then carried the same dial: when every scheme fails we are
	// simply offline, and striking there would demote every peer during an
	// ordinary outage. One strike per Dial, however many quic addrs timed out.
	if !outcome.QuicTimedOut ||
		outcome.SucceededScheme == "" || outcome.SucceededScheme == transport.Quic {
		return
	}
	if s.penalties.registerDegraded(outcome.PeerId) {
		log.Info("quic demoted after dial timeouts",
			zap.String("peerId", outcome.PeerId),
			zap.String("workingScheme", outcome.SucceededScheme))
	}
}

func (s *service) Snapshot() PenaltySnapshot { return s.penalties.snapshot() }

func (s *service) Seed(snap PenaltySnapshot) { s.penalties.seed(snap) }

func (s *service) Reset() {
	log.Info("transport penalties reset")
	s.penalties.reset()
}

func (s *service) SetObserver(observer func()) { s.penalties.setObserver(observer) }

// IsDialDegraded reports whether a quic dial error means the path swallowed
// our packets; re-exported so callers need not import the transport.
func IsDialDegraded(err error) bool { return quic.IsDialDegraded(err) }
