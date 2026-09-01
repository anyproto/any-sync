package peerservice

import (
	"sync"
	"time"
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
}

// PenaltySnapshot is a copy of the whole penalty state, for persistence.
type PenaltySnapshot struct {
	Peers map[string]PeerPenalty `json:"peers"`
}

// transportPenalties tracks, per peer, evidence that QUIC connections to it
// keep dying under DPI-style degradation, and decides when to dial the peer
// (or everyone) yamux-first.
type transportPenalties struct {
	mu sync.Mutex
	// enabled gates the whole mechanism: demotion is opt-in for clients (via
	// PeerService.EnableQuicDemotion) so server nodes keep their dial
	// behavior unchanged.
	enabled    bool
	peers      map[string]PeerPenalty
	now        func() time.Time
	isNodePeer func(peerId string) bool
	observer   func()
}

func newTransportPenalties(now func() time.Time, isNodePeer func(peerId string) bool) *transportPenalties {
	return &transportPenalties{
		peers:      map[string]PeerPenalty{},
		now:        now,
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

func (t *transportPenalties) enable() {
	t.mu.Lock()
	t.enabled = true
	t.mu.Unlock()
}

// registerDegraded records a degraded QUIC outcome for the peer and demotes it
// once the strikes reach the threshold. Returns true when this event demoted
// the peer (for logging).
func (t *transportPenalties) registerDegraded(peerId string) (demoted bool) {
	t.mu.Lock()
	if !t.enabled {
		t.mu.Unlock()
		return false
	}
	s := t.peers[peerId]
	s.ConsecutiveDegraded++
	if s.ConsecutiveDegraded >= demoteThreshold {
		s.DemotedUntil = t.now().Add(demotionTTL(s.BackoffLevel))
		s.BackoffLevel++
		// keep one strike of memory: after the TTL expires, a single further
		// degraded death re-demotes immediately
		s.ConsecutiveDegraded = demoteThreshold - 1
		demoted = true
	}
	t.peers[peerId] = s
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
	observer := t.observer
	t.mu.Unlock()
	if existed && observer != nil {
		observer()
	}
}

// quicDemoted reports whether the peer should be dialed yamux-first: either
// the peer itself is demoted, or enough network nodes are demoted to treat
// the whole network as QUIC-hostile.
func (t *transportPenalties) quicDemoted(peerId string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.enabled {
		return false
	}
	now := t.now()
	if t.peers[peerId].DemotedUntil.After(now) {
		return true
	}
	var demotedNodes int
	for id, s := range t.peers {
		if s.DemotedUntil.After(now) && t.isNodePeer(id) {
			demotedNodes++
			if demotedNodes >= globalDemotionMinPeers {
				return true
			}
		}
	}
	return false
}

func (t *transportPenalties) snapshot() PenaltySnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	peers := make(map[string]PeerPenalty, len(t.peers))
	for id, s := range t.peers {
		peers[id] = s
	}
	return PenaltySnapshot{Peers: peers}
}

// seed replaces the state with a previously stored snapshot.
func (t *transportPenalties) seed(snap PenaltySnapshot) {
	t.mu.Lock()
	t.peers = make(map[string]PeerPenalty, len(snap.Peers))
	for id, s := range snap.Peers {
		t.peers[id] = s
	}
	t.mu.Unlock()
}

// reset drops all penalty state (e.g. on a network change).
func (t *transportPenalties) reset() {
	t.mu.Lock()
	changed := len(t.peers) > 0
	t.peers = map[string]PeerPenalty{}
	observer := t.observer
	t.mu.Unlock()
	if changed && observer != nil {
		observer()
	}
}

func demotionTTL(backoffLevel int) time.Duration {
	ttl := demotionBaseTTL
	for i := 0; i < backoffLevel && ttl < demotionMaxTTL; i++ {
		ttl *= 2
	}
	return min(ttl, demotionMaxTTL)
}
