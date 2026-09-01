package quicdemotion

import (
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// demoteThreshold is the number of consecutive degraded QUIC outcomes
	// (young idle-timeout deaths or handshake timeouts) after which a peer is
	// dialed yamux-first. One event can be bad luck; two in a row on fresh
	// 4-tuples is the DPI signature.
	demoteThreshold = 2
	// demotionBaseTTL is how long the first demotion lasts. Cheap to be
	// wrong: expiry costs one QUIC probe.
	demotionBaseTTL = 30 * time.Minute
	// demotionMaxTTL caps the exponential backoff of repeated demotions.
	demotionMaxTTL = 4 * time.Hour
	// demotionProbeInterval is how often a demoted peer is dialed quic-first
	// anyway, to find out whether the path recovered. Dialing stops at the
	// first working scheme, so without this a demotion silently suppresses
	// the only evidence that could lift it.
	demotionProbeInterval = 10 * time.Minute
	// demotionJitter is the fraction by which a demotion is randomly
	// extended. A deterministic ladder makes the interval to the next quic
	// probe a stable signal an on-path observer can read back - roughly how
	// many degradation episodes this install has seen - and lines the probes
	// of many devices up with each other.
	demotionJitter = 0.25
	// fallbackWindow is how long a failed yamux dial keeps demotion
	// suspended. The verdict is evidence about the network we are on right
	// now, and in the case this feature targets it can never be refreshed:
	// the quic dial keeps succeeding, so yamux is never dialed again and
	// nothing would clear a permanent flag.
	fallbackWindow = 5 * time.Minute
	// strikeWindow is how long a degraded outcome stays relevant. Past it a
	// peer starts from a clean slate: without decay, a peer that had one bad
	// night stayed permanently one strike away from demotion and its entry
	// was kept - and persisted - for the life of the install.
	strikeWindow = time.Hour
	// globalDemotionMinPeers is how many concurrently demoted network nodes
	// it takes to treat the whole network as QUIC-hostile and dial every peer
	// yamux-first. Only nodeconf peers count: a LAN peer going to sleep also
	// produces young idle-timeout deaths.
	globalDemotionMinPeers = 2
)

// PeerPenalty is the per-peer QUIC penalty state. Exported for persistence:
// clients store the snapshot across restarts so a freshly opened app on a
// QUIC-hostile network doesn't have to re-learn the demotion.
type PeerPenalty struct {
	// ConsecutiveDegraded counts degraded outcomes since the last healthy
	// one. While demoted (and after the demotion expires) it stays at
	// demoteThreshold-1, so a single further degraded death re-demotes
	// immediately; only a healthy connection clears the memory.
	ConsecutiveDegraded int `json:"consecutiveDegraded"`
	// DemotedUntil is the wall-clock end of the demotion; zero when the peer
	// has strikes but is not demoted.
	DemotedUntil time.Time `json:"demotedUntil,omitzero"`
	// BackoffLevel is how many demotions the peer has accumulated; each one
	// doubles the next TTL.
	BackoffLevel int `json:"backoffLevel"`
	// LastStrikeAt dates the most recent degraded outcome, so stale evidence
	// can decay out of strikeWindow.
	LastStrikeAt time.Time `json:"lastStrikeAt,omitzero"`
	// NextProbeAt is when this peer is next dialed quic-first despite the
	// demotion, to test whether the path came back.
	NextProbeAt time.Time `json:"nextProbeAt,omitzero"`
}

// PenaltySnapshotVersion is the schema version of PenaltySnapshot. The shape
// is owned by this package but persisted by clients, so it travels with the
// data and a snapshot written by a different version is discarded rather than
// misread.
const PenaltySnapshotVersion = 1

// PenaltySnapshot is a copy of the whole penalty state, for persistence.
type PenaltySnapshot struct {
	Version int                    `json:"version"`
	Peers   map[string]PeerPenalty `json:"peers"`
}

// transportPenalties tracks, per peer, evidence that QUIC connections to it
// keep dying under DPI-style degradation, and decides when to dial the peer
// (or everyone) yamux-first.
type transportPenalties struct {
	mu    sync.Mutex
	peers map[string]PeerPenalty
	// globalUntil is when the network-wide demotion lapses: the second-latest
	// demotion deadline among network nodes, i.e. the moment fewer than
	// globalDemotionMinPeers of them are still demoted. Cached because every
	// dial consults it; recomputed whenever the peer set changes.
	globalUntil time.Time
	// fallbackFailedAt dates the last failed yamux dial to a network node.
	// Demotion is suspended for fallbackWindow after it: preferring a
	// transport we have just watched fail cannot help, and on a censored
	// network it would hand the adversary the choice of our transport.
	fallbackFailedAt time.Time
	// globalNextProbe is the probe clock for the network-wide demotion, which
	// applies to peers that have no entry of their own
	globalNextProbe time.Time
	now             func() time.Time
	// jitter returns a value in [0,1); injectable so tests get exact TTLs
	jitter     func() float64
	isNodePeer func(peerId string) bool
	observer   func()
}

func newTransportPenalties(now func() time.Time, isNodePeer func(peerId string) bool) *transportPenalties {
	return &transportPenalties{
		peers:      map[string]PeerPenalty{},
		now:        now,
		jitter:     rand.Float64,
		isNodePeer: isNodePeer,
	}
}

// setObserver registers a callback fired after every state mutation (outside
// the lock), so a client can persist the snapshot.
func (t *transportPenalties) setObserver(observer func()) {
	t.mu.Lock()
	t.observer = observer
	t.mu.Unlock()
}

// registerDegraded records a degraded QUIC outcome for the peer and demotes it
// once the strikes reach the threshold. Returns true when this event demoted
// the peer (for logging).
func (t *transportPenalties) registerDegraded(peerId string) (demoted bool) {
	t.mu.Lock()
	now := t.now()
	s := t.peers[peerId]
	if !s.LastStrikeAt.IsZero() && now.Sub(s.LastStrikeAt) > strikeWindow {
		// evidence from another era: start over rather than adding to it
		s.ConsecutiveDegraded = 0
		s.BackoffLevel = 0
	}
	s.LastStrikeAt = now
	s.ConsecutiveDegraded++
	if s.ConsecutiveDegraded >= demoteThreshold {
		s.DemotedUntil = now.Add(t.demotionTTL(s.BackoffLevel))
		s.NextProbeAt = now.Add(demotionProbeInterval)
		s.BackoffLevel++
		// keep one strike of memory: after the TTL expires, a single further
		// degraded death re-demotes immediately
		s.ConsecutiveDegraded = demoteThreshold - 1
		demoted = true
	}
	t.peers[peerId] = s
	t.recomputeGlobalLocked()
	observer := t.observer
	t.mu.Unlock()
	if observer != nil {
		observer()
	}
	return demoted
}

// registerHealthy clears the peer's penalty state: the path has proven itself.
func (t *transportPenalties) registerHealthy(peerId string) {
	t.mu.Lock()
	_, existed := t.peers[peerId]
	delete(t.peers, peerId)
	if existed {
		t.recomputeGlobalLocked()
	}
	observer := t.observer
	t.mu.Unlock()
	if existed && observer != nil {
		observer()
	}
}

// quicDemoted reports whether the peer should be dialed yamux-first: either
// the peer itself is demoted, or enough network nodes are demoted to treat
// the whole network as QUIC-hostile.
// quicDemoted reports the stored verdict alone, without the probe or the
// fallback gate that decide an actual dial. Dialing goes through demoteDial;
// this is the state query, used to inspect and assert on the verdict itself.
func (t *transportPenalties) quicDemoted(peerId string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.pruneLocked(now)
	return t.peers[peerId].DemotedUntil.After(now) || t.globalUntil.After(now)
}

// demoteDial decides whether one dial should put yamux ahead of quic. It is
// quicDemoted plus the probe: every demotionProbeInterval a single dial is
// let through quic-first to find out whether the path recovered. Without that
// a demotion cannot be falsified - dialing stops at the first working scheme,
// so a demoted peer never tries quic, never produces a healthy connection,
// and stays demoted for the whole backoff after the path comes back.
func (t *transportPenalties) demoteDial(peerId string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.fallbackFailedAt.IsZero() && t.now().Sub(t.fallbackFailedAt) <= fallbackWindow {
		return false
	}
	now := t.now()
	t.pruneLocked(now)
	if s := t.peers[peerId]; s.DemotedUntil.After(now) {
		if t.dueForProbe(&s.NextProbeAt, now) {
			t.peers[peerId] = s
			return false
		}
		return true
	}
	if t.globalUntil.After(now) {
		return !t.dueForProbe(&t.globalNextProbe, now)
	}
	return false
}

// dueForProbe reports whether it is time to let one dial try quic again,
// rearming the clock when it is.
func (t *transportPenalties) dueForProbe(next *time.Time, now time.Time) bool {
	if next.IsZero() || now.Before(*next) {
		return false
	}
	*next = now.Add(demotionProbeInterval)
	return true
}

// recomputeGlobalLocked refreshes the network-wide demotion deadline. Only
// network nodes count: a LAN peer going to sleep produces the same young
// idle-timeout death, and a phone is not evidence about the network.
func (t *transportPenalties) recomputeGlobalLocked() {
	prev := t.globalUntil
	deadlines := make([]time.Time, 0, len(t.peers))
	for id, s := range t.peers {
		if s.DemotedUntil.IsZero() || !t.isNodePeer(id) {
			continue
		}
		deadlines = append(deadlines, s.DemotedUntil)
	}
	if len(deadlines) < globalDemotionMinPeers {
		t.globalUntil = time.Time{}
	} else {
		slices.SortFunc(deadlines, func(a, b time.Time) int { return b.Compare(a) })
		// the moment fewer than globalDemotionMinPeers nodes are still demoted
		t.globalUntil = deadlines[globalDemotionMinPeers-1]
	}
	// arm the probe only for a demotion that is actually in effect: an
	// expired entry can push globalUntil forward without demoting anything
	now := t.now()
	if t.globalUntil.After(prev) && t.globalUntil.After(now) && t.globalNextProbe.Before(now) {
		t.globalNextProbe = now.Add(demotionProbeInterval)
	}
}

// pruneLocked drops peers that can no longer affect a decision: their
// demotion has run out and their last strike has decayed. Keeps the map - and
// the file clients persist it to - proportional to recent trouble rather than
// to every peer ever dialed. Never removes a demoted peer, so the cached
// global deadline stays valid.
func (t *transportPenalties) pruneLocked(now time.Time) {
	for id, s := range t.peers {
		if s.DemotedUntil.After(now) {
			continue
		}
		if s.LastStrikeAt.IsZero() || now.Sub(s.LastStrikeAt) > strikeWindow {
			delete(t.peers, id)
		}
	}
}

// recordFallback notes the outcome of a yamux dial. Only network nodes count,
// for the same reason they alone drive the network-wide demotion: a LAN peer
// that went to sleep fails its dial constantly and says nothing about whether
// tcp works toward the network. Deliberately global across nodes and never
// persisted - whether tcp works belongs to the network we are on right now.
func (t *transportPenalties) recordFallback(peerId string, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.isNodePeer(peerId) {
		return
	}
	if ok {
		t.fallbackFailedAt = time.Time{}
		return
	}
	t.fallbackFailedAt = t.now()
}

func (t *transportPenalties) snapshot() PenaltySnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(t.now())
	peers := make(map[string]PeerPenalty, len(t.peers))
	for id, s := range t.peers {
		peers[id] = s
	}
	return PenaltySnapshot{Version: PenaltySnapshotVersion, Peers: peers}
}

// seed replaces the state with a previously stored snapshot.
func (t *transportPenalties) seed(snap PenaltySnapshot) {
	if snap.Version != PenaltySnapshotVersion {
		log.Warn("ignoring transport penalties from another schema version",
			zap.Int("version", snap.Version), zap.Int("expected", PenaltySnapshotVersion))
		return
	}
	t.mu.Lock()
	t.peers = make(map[string]PeerPenalty, len(snap.Peers))
	for id, s := range snap.Peers {
		t.peers[id] = s
	}
	t.recomputeGlobalLocked()
	t.mu.Unlock()
}

// reset drops all penalty state (e.g. on a network change).
func (t *transportPenalties) reset() {
	t.mu.Lock()
	changed := len(t.peers) > 0
	t.peers = map[string]PeerPenalty{}
	t.globalUntil = time.Time{}
	t.globalNextProbe = time.Time{}
	t.fallbackFailedAt = time.Time{}
	observer := t.observer
	t.mu.Unlock()
	if changed && observer != nil {
		observer()
	}
}

func (t *transportPenalties) demotionTTL(backoffLevel int) time.Duration {
	ttl := demotionBaseTTL
	for i := 0; i < backoffLevel && ttl < demotionMaxTTL; i++ {
		ttl *= 2
	}
	ttl = min(ttl, demotionMaxTTL)
	return ttl + time.Duration(float64(ttl)*demotionJitter*t.jitter())
}
