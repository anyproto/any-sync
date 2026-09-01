package quicdemotion

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type penaltyFixture struct {
	*transportPenalties
	clock       time.Time
	nodeIds     map[string]bool
	observed    int
	nodeLookups int
}

func newPenaltyFixture() *penaltyFixture {
	fx := &penaltyFixture{
		clock:   time.Unix(1000000, 0),
		nodeIds: map[string]bool{},
	}
	fx.transportPenalties = newTransportPenalties(
		func() time.Time { return fx.clock },
		func(peerId string) bool {
			fx.nodeLookups++
			return fx.nodeIds[peerId]
		},
	)
	// deterministic TTLs unless a test opts into jitter
	fx.jitter = func() float64 { return 0 }
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
	t.Run("TTL is jittered so probes do not line up", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.jitter = func() float64 { return 1 }
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")

		fx.advance(demotionBaseTTL + time.Second)
		assert.True(t, fx.quicDemoted("p1"), "jitter must extend the demotion past the bare base TTL")
		fx.advance(time.Duration(float64(demotionBaseTTL) * demotionJitter))
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

func TestTransportPenalties_GlobalDemotionIsCached(t *testing.T) {
	fx := newPenaltyFixture()
	fx.nodeIds["n1"] = true
	fx.registerDegraded("n1")
	fx.registerDegraded("n1")

	before := fx.nodeLookups
	for i := 0; i < 100; i++ {
		fx.quicDemoted("someOtherPeer")
	}
	assert.Equal(t, before, fx.nodeLookups,
		"every dial consults this: the demoted-node set must be cached, not rescanned under the lock")
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

// TestPeerService_EnableQuicDemotionBeforeStart pins the production call
// order: heart enables demotion from its config component, whose Init runs
// before peerService.Init. The call must not panic, must survive Init, and
// must still end up with the conn observer registered.

func TestTransportPenalties_Decay(t *testing.T) {
	t.Run("strikes outside the window do not accumulate", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.advance(strikeWindow + time.Second)
		fx.registerDegraded("p1")
		assert.False(t, fx.quicDemoted("p1"), "two strikes an era apart are not evidence of a broken path")
	})
	t.Run("strikes inside the window still accumulate", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.advance(strikeWindow - time.Minute)
		fx.registerDegraded("p1")
		assert.True(t, fx.quicDemoted("p1"))
	})
	t.Run("decayed entries are pruned", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.advance(strikeWindow + time.Second)
		assert.False(t, fx.quicDemoted("p1"))
		assert.Empty(t, fx.snapshot().Peers, "state that can no longer affect a decision must not be kept or persisted")
	})
	t.Run("an active demotion is never pruned", func(t *testing.T) {
		fx := newPenaltyFixture()
		// as if seeded from disk after a long gap: the strike is ancient but
		// the demotion still has hours left to run
		fx.seed(PenaltySnapshot{Version: PenaltySnapshotVersion, Peers: map[string]PeerPenalty{
			"p1": {
				ConsecutiveDegraded: demoteThreshold - 1,
				LastStrikeAt:        fx.clock.Add(-24 * time.Hour),
				DemotedUntil:        fx.clock.Add(2 * time.Hour),
				BackoffLevel:        2,
			},
		}})
		assert.True(t, fx.quicDemoted("p1"))
		assert.NotEmpty(t, fx.snapshot().Peers)
	})
}

func TestTransportPenalties_Probe(t *testing.T) {
	// Without a probe a demotion cannot be falsified: dialing stops at the
	// first working scheme, so a demoted peer never tries quic again, never
	// produces a healthy connection, and stays demoted for the full backoff
	// even after the path recovers - or after an adversary stops interfering.
	t.Run("a demoted peer periodically retries quic", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		assert.True(t, fx.demoteDial("p1"))

		fx.advance(demotionProbeInterval + time.Second)
		assert.False(t, fx.demoteDial("p1"), "one dial must be allowed to test quic again")
		assert.True(t, fx.demoteDial("p1"), "and only one: the rest keep using the fallback")
		assert.True(t, fx.quicDemoted("p1"), "the demotion itself is unchanged by a probe")
	})
	t.Run("the network-wide demotion is probed too", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.nodeIds["n1"], fx.nodeIds["n2"] = true, true
		for _, id := range []string{"n1", "n2"} {
			fx.registerDegraded(id)
			fx.registerDegraded(id)
		}
		assert.True(t, fx.demoteDial("otherPeer"))

		fx.advance(demotionProbeInterval + time.Second)
		assert.False(t, fx.demoteDial("otherPeer"))
		assert.True(t, fx.demoteDial("otherPeer"))
	})
}

func TestTransportPenalties_FallbackEvidence(t *testing.T) {
	// Demotion moves traffic onto yamux, so it is only worth doing while
	// yamux actually works. An on-path adversary controls whether quic dies;
	// if it also blocks tcp, preferring yamux just picks the transport the
	// adversary filters better.
	t.Run("no demotion while the fallback transport is failing", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		assert.True(t, fx.demoteDial("p1"))

		fx.recordFallback(false)
		assert.False(t, fx.demoteDial("p1"), "demoting onto a transport we just watched fail helps nobody")
		assert.True(t, fx.quicDemoted("p1"), "the verdict is kept, only its effect is suspended")

		fx.recordFallback(true)
		assert.True(t, fx.demoteDial("p1"))
	})
	t.Run("demotion applies before anything is known about the fallback", func(t *testing.T) {
		// a seeded verdict must survive a restart, when no dial has happened yet
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		fx.registerDegraded("p1")
		assert.True(t, fx.demoteDial("p1"))
	})
}

func TestTransportPenalties_SnapshotVersion(t *testing.T) {
	// the shape of a snapshot is owned by this package but persisted by
	// clients, so a future change here must not be read back as valid state
	t.Run("snapshots carry the current version", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.registerDegraded("p1")
		assert.Equal(t, PenaltySnapshotVersion, fx.snapshot().Version)
	})
	t.Run("a snapshot from another version is discarded", func(t *testing.T) {
		fx := newPenaltyFixture()
		fx.seed(PenaltySnapshot{
			Version: PenaltySnapshotVersion + 1,
			Peers: map[string]PeerPenalty{
				"p1": {ConsecutiveDegraded: demoteThreshold, DemotedUntil: fx.clock.Add(time.Hour)},
			},
		})
		assert.False(t, fx.quicDemoted("p1"))
		assert.Empty(t, fx.snapshot().Peers)
	})
}
