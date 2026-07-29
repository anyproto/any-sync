package pubsub

import (
	"context"
	"time"

	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/crypto"
)

// Handler receives a decrypted, signature-verified message on a subscribed topic.
// Handlers run on a bounded dispatch queue and must not block.
type Handler func(spaceId, topic string, identity crypto.PubKey, payload []byte)

// MembershipChecker gates subscribe and publish on space membership.
// Implementations resolve the space's ACL state; any non-nil error rejects.
type MembershipChecker interface {
	CheckMember(ctx context.Context, spaceId string, identity crypto.PubKey) error
}

// Crypto encrypts and decrypts payloads with the space read key.
// A nil Crypto means plaintext payloads (keyless spaces).
type Crypto interface {
	// Encrypt returns the key id used and the ciphertext.
	Encrypt(spaceId string, payload []byte) (keyId string, encrypted []byte, err error)
	// Decrypt resolves keyId to a historical read key and decrypts.
	Decrypt(spaceId, keyId string, encrypted []byte) ([]byte, error)
}

// PeerProvider resolves the peers a client sends publishes and interest to for a
// space: the responsible sync node plus any directly connected LAN peers.
type PeerProvider interface {
	SpacePeers(ctx context.Context, spaceId string) ([]peer.Peer, error)
}

// Relay is implemented only on responsible nodes; nil on clients.
type Relay interface {
	// IsResponsible reports whether this node is responsible for the space.
	IsResponsible(spaceId string) bool
	// IsResponsibleNode reports whether peerId is a responsible node for the space
	// (used to authorize inbound relayed publishes).
	IsResponsibleNode(spaceId, peerId string) bool
	// OtherResponsiblePeers returns the other responsible nodes, excluding self.
	OtherResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error)
}

// StatusHandler observes Status frames received from serving peers (rejections).
type StatusHandler func(peerId string, status *pubsubproto.Status)

// Deps carries the pluggable pieces wired by the node or client host.
type Deps struct {
	Membership MembershipChecker
	Crypto     Crypto
	Peers      PeerProvider // client side; may be nil on nodes
	Relay      Relay        // node side; nil on clients
	OnStatus   StatusHandler
	// Metric, if set, registers the private pool's prometheus metrics so a node
	// relaying at scale is observable. Optional.
	Metric metric.Metric
	Config Config
}

// Config bounds the engine per DESIGN.md §9; zero values take defaults.
type Config struct {
	MaxPayloadSize       int
	MaxPatternsPerStream int
	MaxPatternsPerSpace  int
	PublishRps           float64
	PublishBurst         int
	WriteQueueSize       int
	DispatchQueueSize    int
	DedupSize            int
	// MaxTimestampSkew bounds how stale (or future) a received message's timestamp
	// may be before it is dropped, raising the replay bar even after dedup eviction.
	MaxTimestampSkew time.Duration
	// ResyncInterval is how often a client re-pushes its interest to space peers,
	// keeping server-side interest alive across reconnects.
	ResyncInterval time.Duration
	// PeerTTL keeps a pubsub stream's peer from being reaped by idle pool GC.
	PeerTTL time.Duration
	// DialQueueWorkers/DialQueueSize size the pool's outbound dial pool.
	DialQueueWorkers int
	DialQueueSize    int
}

func (c Config) withDefaults() Config {
	if c.MaxPayloadSize <= 0 {
		c.MaxPayloadSize = 64 * 1024
	}
	if c.MaxPatternsPerStream <= 0 {
		c.MaxPatternsPerStream = 1000
	}
	if c.MaxPatternsPerSpace <= 0 {
		c.MaxPatternsPerSpace = 100
	}
	if c.PublishRps <= 0 {
		c.PublishRps = 30
	}
	if c.PublishBurst <= 0 {
		c.PublishBurst = 60
	}
	if c.WriteQueueSize <= 0 {
		c.WriteQueueSize = 100
	}
	if c.DispatchQueueSize <= 0 {
		c.DispatchQueueSize = 100
	}
	if c.DedupSize <= 0 {
		c.DedupSize = 4096
	}
	if c.MaxTimestampSkew <= 0 {
		c.MaxTimestampSkew = 5 * time.Minute
	}
	if c.ResyncInterval <= 0 {
		c.ResyncInterval = 20 * time.Second
	}
	if c.PeerTTL <= 0 {
		c.PeerTTL = time.Hour
	}
	if c.DialQueueWorkers <= 0 {
		c.DialQueueWorkers = 4
	}
	if c.DialQueueSize <= 0 {
		c.DialQueueSize = 100
	}
	return c
}

// streamPoolConfig maps the pubsub config onto the streampool's own config. Only
// the dial knobs are consumed by the pool; per-stream queue size is passed
// explicitly to AddStream/ReadStream via WriteQueueSize.
func (c Config) streamPoolConfig() streampool.StreamConfig {
	return streampool.StreamConfig{
		SendQueueSize:    c.WriteQueueSize,
		DialQueueWorkers: c.DialQueueWorkers,
		DialQueueSize:    c.DialQueueSize,
	}
}
