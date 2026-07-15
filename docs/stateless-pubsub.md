# Space-Scoped Stateless Pub/Sub over any-sync

Status: **IMPLEMENTED (v1)** — shipped in this repo as `commonspace/pubsub` (engine) plus
additions to `net/streampool`. This document is the as-built reference: it describes what
the code does, why the design is shaped this way, and what is deliberately left for
downstream repos or a later version. Grounded against the current tree
(`commonspace/pubsub/`, `net/streampool/`).

Downstream wiring — the any-sync-node relay component and the anytype-heart client — lives
in separate repos and is tracked in [§13](#13-implementation-status).

---

## 1. Summary

An ephemeral, fire-and-forget, at-most-once publish/subscribe channel scoped to a space,
carried over a **dedicated DRPC bidi stream** fully isolated from the sync engine. Topics
are plaintext `/`-separated hierarchies inside a space; subscriptions may use NATS-style
wildcards (`*` one segment, `>` trailing segments). Any space member (Reader/Guest
included) may publish and subscribe. Payloads are end-to-end encrypted with the space
ReadKey and signed with the sender's account key; relay nodes route ciphertext they cannot
read. Fan-out goes through the responsible sync nodes **and** directly to LAN-discovered
peers, with a small bounded msgId dedup cache on receivers. No message is ever persisted;
the only routing state is transient in-memory interest state (stream tags + a pattern trie)
that dies with the connection.

The engine (`commonspace/pubsub.Service`, `CName = "common.commonspace.pubsub"`) is a single
component used by **both** sides: nodes wire it with a `Relay` (relay role), clients wire it
with a `PeerProvider` (client role). One long-lived `PubSubStream` per peer pair multiplexes
all spaces and topics.

### Design decisions

| Question | Decision |
|---|---|
| Meaning of "stateless" | No message persistence anywhere. Transient in-memory interest tables (stream tags + trie) are allowed and used. |
| Stream reuse vs dedicated | Dedicated `PubSub` DRPC service + its own streampool instance; never touches `ObjectSyncStream` or the sync dispatch path. |
| New service vs new RPC on SpaceSync | New service, own proto package — independent versioning, unknown-service fallback for old peers. |
| Topic model | `/`-separated segment hierarchy within a space. Subscriptions may use NATS-style wildcards: `*` matches exactly one segment, `>` matches one-or-more trailing segments (tail-only). Publishers must use fully-qualified topics. Matching runs on a bounded per-space pattern trie at the relay ([§5.1](#51-interest-matching-wildcards)). |
| Topic privacy | Plaintext to relays. Comparable exposure to today's plaintext `objectId`s on the sync path. |
| Subscribe permission | Any space member: `!NoPermissions()` (`commonspace/object/acl/list/models.go:90`). |
| Publish permission | Any space member. Attribution via per-message account-key signature; abuse contained by per-peer rate limits. Plus a reserved **self-owned namespace**: topics `acc/…/<accId>` accept publishes only from `accId` ([§7.2](#72-access-control)). |
| Payload confidentiality | Encrypted client-side with the current space ReadKey + `keyId` indirection. Removed member loses access at next key rotation. |
| Authenticity | Per-message account-key signature, verified by receivers. Protects against a semi-trusted node forging or reattributing messages. |
| Topology | Publisher → its responsible node (+ direct LAN peers). Node relays once to the other responsible nodes (`relayed` flag, never re-forwarded → loop-free). |
| Duplicate suppression | Receiver-side bounded FIFO ring keyed by `msgId` (duplicate paths exist by design: LAN + node). |
| Backpressure | Bounded queues, `TryAdd` drop-on-overflow end to end — the existing streampool discipline. Slow subscriber ⇒ dropped messages, never memory growth. |
| Queue groups (1-of-N) | Non-goal v1. |
| Catch-up / replay / retained messages | Non-goal, by definition of stateless. Reconnect ⇒ resubscribe ⇒ only new messages. |
| Presence lifecycle | Not in the protocol. v1 is a generic app API; presence (heartbeat/timeout/leave, Yjs-awareness style) is an app pattern on top ([Appendix A](#appendix-a--presence-as-an-app-pattern-non-normative)). |

### Requirements compliance

The feature was specified against three hard requirements from the original ask ("users of
the same space can subscribe/publish topics within a space; work effectively over the
any-sync protocol; be memory-effective; respect ACL access"):

- **R1 (on-protocol):** rides existing transports, the secureservice handshake, the DRPC
  mux, and a second streampool instance. A dedicated sub-stream on the existing `MultiConn`
  — no new transport or TLS handshake.
- **R2 (memory-effective):** per-stream bounded `mb.MB` queues with drop-on-overflow
  (`net/streampool/stream.go:32-44`), interest state = stream tags + pattern trie (both
  O(active subscriptions)), a fixed-size dedup FIFO ring, a per-peer publish token bucket,
  and caps on pattern count and topic/payload size. No path grows with message volume or
  offline duration.
- **R3 (ACL-aware):** the serving peer gates subscribe *and* publish on space membership at
  the current ACL head (new, additive enforcement — the sync path has no such check at
  subscribe time); confidentiality holds against the relay via ReadKey encryption; membership
  removal cuts new traffic at key rotation and actively drops subscriptions ([§7.4](#74-confidentiality--membership-change)).

---

## 2. Background & rationale

This section distills the research that shaped the design, for readers who want the *why*
without the original problem-framing document.

### 2.1 Where this sits in the pub/sub landscape

"Stateless pub/sub" here means the **core-NATS / Redis-pub-sub** family: fire-and-forget,
at-most-once, no persistence, full decoupling of publishers and subscribers. The relay may
hold a small *transient* in-memory interest table (as core NATS does) — that is not
"stateful" in the sense that matters; only durable message state is excluded.

| System | Topic model | Wildcards | Delivery | Persistence | Notable for us |
|---|---|---|---|---|---|
| **NATS core** | dot-hierarchy subjects | `*` (1 token), `>` (tail multi) | fan-out + queue-group (1-of-N) | none (JetStream is separate) | the canonical model; subject wildcards; sublist trie |
| **Redis pub/sub** | flat channels + patterns | `*`, `?`, `[..]` glob | fan-out | none | simplest fire-and-forget; subscriber must be connected |
| **MQTT** | `/`-hierarchy topics | `+` (1 level), `#` (tail multi) | QoS 0/1/2 | retained msg + sessions ⇒ *stateful* | wildcard syntax variant; retained message is the anti-pattern to avoid |
| **libp2p GossipSub** | flat topics | none | epidemic mesh fan-out | none | closest to any-sync's decentralization ethos; peer-scoring for abuse |
| **Yjs awareness** | one implicit channel/doc | n/a | state-based CRDT diff | ephemeral, auto-GC | the presence reference: `(clientID, clock, state\|null)`, re-announce/timeout |
| **Phoenix Channels** | `topic:subtopic` strings | none | fan-out; Presence on top | ephemeral | presence = minimal ephemeral metadata over pub/sub |
| **Matrix EDUs** | per-room | n/a | fan-out to room servers | ephemeral | typing/presence as Ephemeral Data Units, distinct from the persistent event DAG |

Three cross-cutting lessons drove the design:

- **Topology is the fork.** Central-broker systems keep an interest table; GossipSub is a
  brokerless mesh. any-sync sits between: clients reach a small set of **responsible sync
  nodes** (semi-trusted relays that store ciphertext they cannot read) and *may* also reach
  other clients directly (LAN). The relay-through-node model is broker-like, but the broker
  is **untrusted for content** — which forces the encryption + signing model in [§7](#7-security-model).
- **Presence is the killer use case**, and it is *always* kept separate from the persistent
  document layer (Yjs awareness, Phoenix Presence, Matrix EDUs). v1 keeps presence out of
  the protocol and provides it as an app recipe ([Appendix A](#appendix-a--presence-as-an-app-pattern-non-normative)).
- **Wildcards cost the relay** a per-message trie walk; flat topics do not. We accept that
  cost for expressiveness, bounded by per-space pattern caps ([§5.1](#51-interest-matching-wildcards)).

### 2.2 The any-sync substrate we build on

The load-bearing facts about this codebase that the design leans on:

- **StreamPool is the fan-out core.** `net/streampool` caches `drpc.Stream`s, indexes them
  by peer and by **tag**, opens them lazily, and pushes messages onto **bounded** per-stream
  write queues (`mb.MB`, `TryAdd`, drop-on-overflow — `net/streampool/stream.go:32-44`).
  `Broadcast(msg, tags...)` is the existing tag-keyed fan-out primitive; `AddTagsCtx` /
  `RemoveTagsCtx` are the existing dynamic (un)subscribe hooks. Pub/sub instantiates its own
  pool rather than layering onto the sync pool.
- **Responsible nodes via consistent hash.** `NodeIds(spaceId)` returns the tree nodes on
  the chash ring for a space; `ReplicationFactor = 3` ⇒ up to 3 responsible sync nodes per
  space. These are the always-reachable fan-out hubs.
- **ACL is cryptographic, not a per-message live check.** Writes are enforced when a signed
  record is validated on apply; reads are enforced by **encryption** — content is encrypted
  with a per-space `ReadKey` handed only to members, and the key rotates on membership
  change. The permission ladder is `None=0, Owner=1, Admin=2, Writer=3, Reader=4, Guest=5`;
  there is no `CanRead()` — "can read" is `!NoPermissions()`. The shared sync layer performs
  **no** per-message reader-authorization, so pub/sub's ACL gating at subscribe/publish time
  is new, additive enforcement.
- **Don't reuse `ObjectSyncStream`.** Every frame on it is routed into the sync engine and
  shares one bounded queue; multiplexing ephemeral pub/sub there would couple the two
  (head-of-line blocking, intertwined backpressure) and muddle fire-and-forget ephemera with
  DAG anti-entropy. A separate multiplexed DRPC stream is cheap — another sub-stream on the
  existing `MultiConn`, not a new handshake — so pub/sub gets its own stream, tags, and
  queues, fully isolated from sync flow control.

### 2.3 Why the big forks landed where they did

- **Dedicated service, not a new SpaceSync RPC** — independent lifecycle and versioning;
  old peers that don't know `PubSub` fail the stream open cleanly and the client treats the
  peer as pubsub-unavailable.
- **Relay-through-node, not pure mesh** — nodes are the only always-reachable members; mesh
  fits the ethos but has no guaranteed connectivity. LAN peers are used *in addition*, not
  instead.
- **Per-message signing on top of encryption** — encryption alone gives confidentiality but
  not authenticity: a Reader (or the relay) could spoof or reattribute a message that other
  members can still decrypt. Ed25519 sign/verify (~30–80 µs) is negligible at
  ephemeral-signal rates and buys spoof-proof attribution, which the self-owned `acc/`
  namespace then builds on.
- **No queue groups, no interest propagation between nodes in v1** — see [§14](#14-non-goals-v1).

---

## 3. Semantics contract

- **At-most-once.** A publish reaching the relay is copied into bounded per-subscriber
  queues; overflow drops. No acks, no retransmit, no ordering across publishers, no dedup
  beyond the duplicate-path ring.
- **Fire-and-forget.** A publish with no subscribers is discarded. Nothing is stored.
- **Decoupled.** Publishers don't know subscribers. The subscriber set is whoever holds a
  live tagged stream at the instant of fan-out.
- **Subscribe is unacknowledged.** Success is silent ([§6](#6-what-happens-on-each-frame-serving-peer)); interest takes effect when the serving
  peer processes the frame, so messages published concurrently by others may be missed.
  There is no "subscribed as of time T" guarantee — the same race NATS documents for
  cluster-wide subscription visibility, inherent to at-most-once. Apps needing a consistent
  starting state combine pub/sub with a snapshot read (e.g. presence: subscribe, then
  announce yourself, which prompts others' next heartbeat).
- **Reconnect = clean slate.** Subscriptions die with the stream; the client re-subscribes on
  reconnect (via the resync loop, [§9](#9-lifecycle--reconnect)) and sees only new traffic.
- **Echo.** A publisher whose own interest set matches the topic would receive its own
  message back from the relay. The client suppresses this by pre-recording its own `msgId` in
  the dedup ring at publish time and delivering to local handlers directly (bounded,
  drop-on-overflow dispatch queue — so local delivery is best-effort too). Net effect = NATS
  `no_echo` semantics without a wire flag.
- **Duplicates possible in theory** (ring eviction under extreme rates), so payload design
  must be idempotent / last-write-wins at the app level. In practice the ring makes
  duplicates vanishingly rare.

---

## 4. Wire protocol

Proto package `commonspace/pubsub/pubsubproto/protos/pubsub.proto` (generated via the
Makefile pipeline):

```proto
syntax = "proto3";
package pubsub;

service PubSub {
  // One long-lived bidi stream per peer pair, multiplexing all spaces/topics.
  rpc PubSubStream(stream PubSubMessage) returns (stream PubSubMessage);
}

message PubSubMessage {
  oneof content {
    Subscribe   subscribe   = 1;
    Unsubscribe unsubscribe = 2;
    Publish     publish     = 3;
    Status      status      = 4;
  }
}

// Delta semantics: adds topic patterns to the stream's interest set for spaceId.
// Patterns may contain wildcards: '*' (one segment), '>' (trailing segments, tail-only).
message Subscribe {
  string spaceId = 1;
  repeated string topics = 2;
}

// Removes patterns (matched verbatim against the interest set, not expanded);
// empty topics = remove all patterns of spaceId.
message Unsubscribe {
  string spaceId = 1;
  repeated string topics = 2;
}

message Publish {
  string spaceId         = 1;
  string topic           = 2;  // fully-qualified; wildcards not allowed
  bytes  msgId           = 3;  // 16 random bytes, generated by publisher (dedup key)
  string keyId           = 4;  // space ReadKey id used for payload; "" = plaintext (keyless spaces)
  bytes  payload         = 5;  // ciphertext (or plaintext iff keyId == "")
  bytes  identity        = 6;  // sender account pubkey (marshalled)
  bytes  signature       = 7;  // account-key sig, see §7.3
  int64  timestampMilli  = 8;  // sender wall clock, informational
  bool   relayed         = 9;  // set by a node when forwarding node→node; never re-forwarded; excluded from signature
}

// Sent by the serving peer on a rejected subscribe/publish. Success is silent.
message Status {
  string spaceId = 1;
  repeated string topics = 2;   // echo of the offending request (topic of a publish goes here too)
  ErrCodes code = 3;
  bytes msgId = 4;              // echoes a rejected publish's id for correlation; empty for subscribe rejections
}

enum ErrCodes {
  Unexpected = 0;          // zero value: a zeroed Status reads as an error, not success
  NotAMember = 1;          // identity has no permissions in the space's ACL
  NotResponsible = 2;      // the serving node is not responsible for the space
  RateLimited = 3;
  TooManyTopics = 4;
  InvalidMessage = 5;      // malformed frame, oversized payload, identity mismatch, bad signature
  TopicNotOwned = 6;       // publish into acc/…/<accId> by an identity other than accId
  InvalidTopic = 7;        // malformed topic/pattern: bad wildcard placement, reserved chars, non-canonical form
  ErrorOffset = 1100;      // core bands 100..900 are full; pubsub takes 1100 (see docs/rpc-error-offsets.md)
}
```

Constraints enforced by the serving peer (config defaults, [§10](#10-configuration--defaults)). A rejected
Subscribe/Publish gets a `Status` and is otherwise ignored — the stream **stays open** (the
NATS precedent: a limit violation is an error reply, not a disconnect):

- **topic:** UTF-8, 1..256 bytes, `/`-separated segments (≤16 segments, no empty segments);
  recommended segment charset alnum + `.-_`. Canonical form has **no leading `/`** (`acc/x`
  and `/acc/x` would silently differ — rejected as `InvalidTopic`). The 256-byte limit is a
  compile-time constant (`topic.go`), not a config knob.
- **wildcards (subscription patterns only):** `*` matches exactly one segment
  (`chat/*/typing` ⇒ `chat/abc/typing`, not `chat/typing` or `chat/a/b/typing`); `>` matches
  one-or-more trailing segments and is valid only as the final segment (`chat/>` ⇒ everything
  under `chat/`). Wildcards must be complete segments (`chat/ty*` is invalid). A `Publish`
  topic containing `*` or `>` is rejected — publishers always use fully-qualified topics.
- **reserved self-owned namespace:** a topic whose first segment is `acc` is publishable
  **only** by the account whose id equals the topic's *last* segment — e.g.
  `acc/online/<accId>`, `acc/cursor/<accId>`. Subscribe stays open to all members (including
  patterns like `acc/online/*`). Violations ⇒ `Status{TopicNotOwned}`.
- **payload:** ≤ 64 KiB (enforced after DRPC decode).
- **per-stream interest set:** ≤ 100 patterns per space, ≤ 1000 total.
- **identity in a Publish must equal the connection-context identity** (`net/peer/context.go:88`)
  when arriving from a client (`relayed == false`). This binds attribution to the
  handshake-proven account without the node verifying the signature per message.

Interest tag format inside the pool: `spaceId + "/" + pattern`, verbatim (spaceId contains no
`/`; first separator wins). Wildcard resolution happens in the pubsub engine's matcher, not
in the pool ([§5.1](#51-interest-matching-wildcards)).

---

## 5. Topology & relay rules

```
  publisher client ──publish──▶ responsible node A ──relayed=true──▶ node B ──▶ its subscribers
        │                            │                              node C ──▶ its subscribers
        │                            └──▶ A's local subscribers (tag fan-out)
        └──────direct publish──▶ LAN peers (same space, discovered via mDNS)
```

1. **Clients never forward.** A client receiving a `Publish` (from a node or a LAN peer)
   delivers it locally only.
2. **A node forwards only client-originated messages** (`relayed == false` arriving on a
   client stream): it stamps `relayed = true` and sends one copy to each *other* responsible
   node for the space, plus fans out to its local subscribers via the tag index.
3. **A node never forwards `relayed == true`** — it only fans out locally, and only if the
   sending peer is itself a responsible node for the space (`Relay.IsResponsibleNode`). With
   `ReplicationFactor = 3`, every message traverses at most client → node → node, hop limit
   2, loop-free without any dedup state on nodes. (This is the NATS cluster rule: messages
   from a route are distributed only to local clients.)
4. **A node rejects subscribe/publish for spaces it is not responsible for**
   (`Status{NotResponsible}`, via `Relay.IsResponsible`).
5. **LAN peers are symmetric.** Both sides run the same pubsub component; a LAN peer's stream
   carries Subscribe frames like a node's does, and publishes to LAN peers go direct. Clients
   apply the member-check on LAN subscribes the same way nodes do (they hold the ACL).

Duplicate paths are expected (a subscriber may get the same message from a LAN peer and from
its node). The **receiver** suppresses via a fixed-size FIFO ring keyed by `msgId` (default
4096 entries). Publishers self-suppress echoes by msgId too ([§3](#3-semantics-contract), Echo).

Client → node selection piggybacks on the host's existing responsible-peer choice (one node
at a time), so the pubsub stream goes to the same node the client already syncs with.

### 5.1 Interest matching (wildcards)

The streampool tag index is exact-match (`streamIdsByTag`, `net/streampool/streampool.go`),
so the pubsub engine layers a matcher on top rather than replacing the pool:

- Each accepted subscription registers its **pattern string verbatim as the stream tag**
  (`spaceId + "/" + pattern`) — the pool keeps doing stream bookkeeping (add/remove/GC on
  stream close) exactly as today.
- In parallel, the engine maintains a **per-space segment trie** of live patterns, mirroring
  the NATS sublist shape (`trie.go`): one level per segment, a `map[segment]` for literals
  plus two dedicated wildcard slots per level (`pwc` for `*`, `fwc` for `>`), each terminal
  holding a refcount of subscribing streams. Match order follows NATS `matchLevel`: at each
  level add `fwc` matches, branch through `pwc`, hash-lookup the literal.
- On publish to concrete topic `T`: walk the trie with `T`'s segments, collecting every
  matching pattern (O(segments × matched branches), segments ≤ 16). Then fan out once per
  matched pattern tag, **deduplicating stream ids across patterns**: a stream subscribed to
  both `chat/>` and `chat/*/typing` receives one copy, not two. `streampool.Broadcast` dedups
  stream ids across the tag set it is given (it builds a `seen` set when handed more than one
  tag — `net/streampool/streampool.go:411-439`), so the engine passes all matched pattern
  tags to a single `Broadcast` call and the pool guarantees one copy per stream.
- Trie cleanup: refcount decrement on Unsubscribe; on stream close the engine reconciles via
  the pool's `WithStreamCloseHook` callback (`net/streampool/streampool.go:43`), keyed by
  `streamId`, dropping the stream's patterns from the trie.
- Exact-topic subscriptions are just patterns without wildcard segments — one code path.

The trie is bounded by the same caps as the interest set (≤ 1000 patterns/stream, ≤ 100/space/
stream), so relay memory stays O(active subscriptions), and matching cost is paid only per
publish within that space.

**Deliberately no match-result cache.** NATS fronts its sublist with a literal-subject →
result cache, but its own history (`<0.5%` hit rates, lock contention, latency spikes under
sub/unsub churn) led to `NoCache` sublists for exactly our analog: small, churny,
per-connection tries. Our tries are per-space (small) and ephemeral interest is churny, so a
direct walk of a ≤ 16-level trie beats a cache we'd constantly flush. Revisit only with
profiling evidence.

---

## 6. What happens on each frame (serving peer)

**Subscribe** — validate `spaceId` (non-empty, no `/`); resolve space ACL state; reject with
`Status{NotResponsible}` if the node isn't responsible; check membership
(`MembershipChecker.CheckMember`); validate patterns (`InvalidTopic` on bad wildcard
placement / reserved chars / non-canonical form); check pattern-count caps; then register in
the trie and add the tags to the stream. Interest and tag are committed under one lock with
rollback if the stream vanished mid-subscribe. Reject ⇒ `Status`, no tag.

**Unsubscribe** — remove tags + trie refcount decrement. No checks needed.

**Publish (from a client stream, `relayed == false`)** —
1. Size/shape checks (a topic containing `*`/`>` ⇒ `InvalidTopic`); `identity == ctxIdentity`
   with a non-empty guard; responsibility check; membership check against cached ACL state;
   self-owned-namespace check (topic `acc/…` ⇒ last segment must equal the sender's account —
   one string compare); per-peer token bucket ([§8](#8-memory--abuse-bounds)). A rejected publish returns a
   `Status` (rate-limited publishes are notified per rejection, not once per window).
2. Match the topic against the space's pattern trie ([§5.1](#51-interest-matching-wildcards)), dedup stream ids across
   matched patterns, then `Broadcast` on the pubsub pool — the existing tag-index fan-out,
   which per-stream `TryAdd`s and drops on overflow.
3. Stamp `relayed = true` and send to the other responsible nodes (`Relay.OtherResponsiblePeers`,
   lazy stream open via the pool).

**Publish (`relayed == true`, from a node stream)** — verify the sending peer is a
responsible node for the space (`Relay.IsResponsibleNode`); fan out locally only (same trie
match). The `identity == ctxIdentity` rule does not apply (the forwarding node is not the
author); authenticity is the receiver's signature check ([§7.3](#73-authenticity)).

**Receive (client)** — the order is cheap-filters-first, verify, then dedup:
local-interest match → membership (`identity` is a member at the local ACL head) → if the
topic is in the `acc/` namespace, check its last segment equals `identity.Account()`
(end-to-end: the signature covers the topic, so not even a malicious relay can inject into
someone else's self-owned topic) → timestamp staleness → **Ed25519 signature verify** →
record `msgId` in the dedup ring → resolve ReadKey by `keyId`, decrypt → dispatch to local
topic handlers on the bounded dispatch queue. Any failure ⇒ drop + debug metric, never an
error to the peer. Recording the ring only *after* verify means a forged-signature flood is
shed cheaply and cannot evict legitimate ids to reopen a replay window.

---

## 7. Security model

### 7.1 Threat model — the semi-trusted relay

The node **can**: observe spaceIds, topics, sender identities, timing, sizes (accepted —
comparable to sync-path metadata today); drop, delay, reorder messages (accepted —
at-most-once contract). The node **cannot**: read payloads (no ReadKey); forge or reattribute
messages (signature); replay effectively. Replay defense is two-layer: the `msgId` dedup ring
catches the short window, and the receiver enforces a **signed-timestamp staleness window**
(`Config.MaxTimestampSkew`, default 5 min) so that even after the ring evicts an id, a relay
replaying an old signed frame is rejected on its stale timestamp. A relay cannot forge a fresh
timestamp — it is covered by the signature.

### 7.2 Access control

Both directions gate on **space membership at the current ACL head**, checked by the serving
peer from its local ACL copy — a cached in-memory lookup, no coordinator round trip. This is
*new* enforcement: today's sync path has none at subscribe time. Publish and subscribe both
require `!NoPermissions()`; there is no `CanWrite` requirement (any member publishes —
presence/typing-style uses need Readers to emit).

**Self-owned topics.** The `acc/` namespace adds a per-topic ownership rule on top of
membership: only the account named by the topic's last segment may publish there. It is
enforced in **three** places — the publisher's own `Publish` call fails fast before sending;
the serving peer rejects it (cheap, because `identity == ctxIdentity` is bound by the
handshake); and every receiver re-checks it (via the signature, which covers the topic
string). This gives apps spoof-proof per-account channels — e.g. `acc/online/<accId>` — where
consumers can trust the topic itself, not just the message attribution. It generalizes the
push-server's "silent self-channel" restriction and matches the proven NATS pattern
(per-identity subject prefixes rather than dynamic per-message grants). Wildcards make the
fan-in side cheap: one `acc/online/*` subscription covers every member's online topic, and
each received message is still individually ownership-checked against its concrete topic and
verified signature.

### 7.3 Authenticity

```
signature = accountKey.Sign(
  "anysync:pubsub:v1" | le32‖spaceId | le32‖topic | le32‖msgId | le32‖keyId |
  le64(timestampMilli) | payload)
```

Each variable-length field is length-prefixed (le32) so field boundaries can't shift (e.g.
`spaceId="ab",topic="c"` vs `spaceId="a",topic="bc"` sign differently); `payload` trails
unprefixed as the final field. `relayed` is excluded (mutated in transit). Receivers verify;
nodes don't need to (attribution from clients is already bound by `identity == ctxIdentity`,
and verifying per-message on the relay buys little at real CPU cost). Ed25519 sign/verify is
~30–80 µs — negligible at ephemeral-signal rates.

Receivers run the cheap filters (local-interest match, membership, `acc/` ownership, timestamp
staleness) *before* the Ed25519 verify, and record the dedup ring only after verify succeeds
([§6](#6-what-happens-on-each-frame-serving-peer)) — so a relay/LAN-peer flood of forged-signature messages is shed cheaply and
cannot evict legitimate ids from the ring to reopen a replay window ([§7.1](#71-threat-model--the-semi-trusted-relay)).

### 7.4 Confidentiality & membership change

Payloads encrypt with the space's current ReadKey, carrying its key id as `keyId`. Receivers
hold historical keys via the ACL, so rotation mid-flight is safe. On member removal the
existing ACL key rotation cuts decryption of new traffic automatically. Additionally the
engine exposes two active-eviction entry points that nodes wire to their ACL-update hook:

- **`RevalidateMembers(spaceId, isMember func(account string) bool)`** — the ACL-change hook.
  On an ACL update the node calls it once; it evicts *every* subscriber of the space whose
  account no longer passes `isMember`, in a single pass.
- **`EvictMember(spaceId, identity)`** — the single-identity variant, for when exactly one
  account is being removed.

Both strip the target's per-stream tags (via `streampool.RemoveTagsById`, so delivery —
which is tag-keyed — stops even while the stream stays open) and decrement the match trie.
Bounded work: one scan of the streams subscribed to that space per ACL change.

Spaces without a ReadKey (keyless/public spaces): `keyId = ""`, plaintext payload, signature
still required. A nil `Crypto` in `Deps` selects this mode.

---

## 8. Memory & abuse bounds

| Resource | Bound | Mechanism |
|---|---|---|
| Outbound per-stream buffer | `WriteQueueSize` msgs (default 100) | `mb.MB` + `TryAdd` drop (`net/streampool/stream.go:39`) |
| Interest table | ≤ 1000 patterns/stream, ≤ 100/space/stream | reject with `TooManyTopics`; state dies with stream |
| Pattern trie ([§5.1](#51-interest-matching-wildcards)) | O(total live patterns × segments), segments ≤ 16 | same caps as interest table; pruned via stream-close hook |
| Publish rate | token bucket **per peer** (default 30 msg/s, burst 60) | checked in the pubsub handler — *inside* the stream, because the RPC limiter only gates stream-open. One bucket per peer (all of a peer's streams share it), so opening more streams can't multiply the budget. Deliberate divergence from core NATS, which has no publish rate limiting; acceptable for trusted clients but not for semi-trusted multi-tenant relays |
| Local dispatch queue | `DispatchQueueSize` msgs (default 100) | `mb.MB`, drop-on-overflow — local handler delivery is best-effort |
| Payload size | ≤ 64 KiB | reject `InvalidMessage` (after DRPC decode) |
| Dedup cache | fixed FIFO ring, 4096 msgIds | evicts oldest, O(1) |
| Fan-out amplification | 1 upload → ≤ 2 node-node copies → N subscriber queues | node-side copy (publisher uploads once); per-destination stamping reuses the streampool copy path |
| Idle streams | closed with the sub-connection; tags GC'd on stream removal; `PeerTTL` keeps a quiet subscriber's peer from idle-GC reaping | existing pool behavior + `Config.PeerTTL` |

No persistence, no unbounded map on the message path, no per-message allocation beyond pooled
message structs.

---

## 9. Lifecycle & reconnect

Because streampool opens streams lazily and never re-sends interest on a fresh stream, a pure
subscriber would go silent after its stream drops (a routine event: idle pool GC reaps quiet
streams). The engine handles this:

- **`SyncInterest(ctx, spaceId)`** re-sends all local interest for a space to its current
  peers. Hosts call it after (re)connecting to a space's peers (e.g. on
  `rebuildResponsiblePeers`).
- A **resync loop** (client role only) re-pushes all local interest every
  `Config.ResyncInterval` (default 20 s) and immediately on a client-side stream close.
  Re-sends are idempotent (the serving side dedups per stream) and bounded by the local
  subscription set. Each pass fires one dial task per local space through the bounded dial
  queue (`DialQueueSize`); a client with more open spaces than that drops the overflow for a
  tick and restores it on a later tick (self-correcting, no leak).
- **`CloseSpace(spaceId)`** drops all local and serving-side interest for a space and
  withdraws local interest from its peers. Hosts call it on space unload so a global service
  doesn't retain closed-space state.

Interest on the serving side is keyed by **streamId, not peerId**: two streams from the same
peer are independent, so a reconnecting peer's fresh stream keeps its interest when the stale
stream closes, and a stream that dies mid-subscribe can't orphan interest.

---

## 10. Configuration & defaults

`pubsub.Config` (zero values take the defaults below, via `Config.withDefaults()`):

| Knob | Field | Default |
|---|---|---|
| max payload | `MaxPayloadSize` | 64 KiB |
| max patterns per stream | `MaxPatternsPerStream` | 1000 |
| max patterns per space | `MaxPatternsPerSpace` | 100 |
| publish rate per peer | `PublishRps` / `PublishBurst` | 30 msg/s / burst 60 |
| per-stream write queue | `WriteQueueSize` | 100 (both client and node) |
| local dispatch queue | `DispatchQueueSize` | 100 |
| dedup ring | `DedupSize` | 4096 msgIds |
| received-timestamp staleness window | `MaxTimestampSkew` | 5 min |
| client interest resync interval | `ResyncInterval` | 20 s |
| pubsub stream peer TTL | `PeerTTL` | 1 h |
| outbound dial pool | `DialQueueWorkers` / `DialQueueSize` | 4 workers / 100 queued |

Max topic length (256 B) and the signature prefix are compile-time constants, not config
fields.

---

## 11. Public API & package layout

### 11.1 `commonspace/pubsub` (the engine)

`Service` is the single component used by both node and client hosts:

```go
type Service interface {
    app.ComponentRunnable

    // Publish encrypts, signs and fire-and-forgets payload to the topic within the space.
    Publish(ctx context.Context, spaceId, topic string, payload []byte) error

    // Subscribe registers a local handler for a pattern and pushes the interest to the
    // space's peers. The returned func unregisters and unsubscribes.
    Subscribe(spaceId, pattern string, h Handler) (unsubscribe func(), err error)

    // SyncInterest (re)sends all local interest for the space to its current peers;
    // call after (re)connecting to a space's peers.
    SyncInterest(ctx context.Context, spaceId string) error

    // CloseSpace drops all local and serving-side interest for the space and withdraws
    // the local interest from its peers.
    CloseSpace(spaceId string)

    // EvictMember drops all serving-side interest of one identity in a space (§7.4).
    EvictMember(spaceId string, identity crypto.PubKey)

    // RevalidateMembers evicts every subscriber of the space whose account no longer
    // passes isMember — the ACL-change hook (§7.4).
    RevalidateMembers(spaceId string, isMember func(account string) bool)

    // HandleStream serves an inbound PubSubStream; blocks for the stream lifetime.
    HandleStream(stream drpc.Stream) error
}

func New(deps Deps) Service

// Handler receives a decrypted, signature-verified message on a subscribed topic.
// Handlers run on a bounded dispatch queue and MUST NOT block.
type Handler func(spaceId, topic string, identity crypto.PubKey, payload []byte)
```

`Deps` carries the pluggable pieces the host wires in:

| Field | Role | Node | Client |
|---|---|---|---|
| `Membership MembershipChecker` | gate subscribe/publish on ACL membership | ✓ | ✓ (LAN serving) |
| `Crypto Crypto` | ReadKey encrypt/decrypt; nil ⇒ plaintext (keyless spaces) | — | ✓ |
| `Peers PeerProvider` | resolve a space's peers (responsible node + LAN) | — | ✓ |
| `Relay Relay` | responsibility + other-node resolution; nil on clients | ✓ | — |
| `OnStatus StatusHandler` | observe rejection `Status` frames from serving peers | optional | optional |
| `Metric metric.Metric` | register the private pool's prometheus gauges | optional | optional |
| `Config Config` | bounds ([§10](#10-configuration--defaults)) | ✓ | ✓ |

`RegisterRpc(mux, service)` registers the `PubSub` DRPC service on a node's mux.

### 11.2 `net/streampool` additions

The pubsub engine reuses the streampool but needs a **second, independent instance**, so the
pool grew a few seams (all backward compatible; the sync-side pool is untouched):

- **`NewStreamPool(handler streamhandler.StreamHandler, cfg StreamConfig, opts ...Option) StreamPool`**
  — a non-component constructor. The existing component `New()` remains the base; the pubsub
  service embeds its own pool with its own handler, queues, and tags.
- **`WithStreamCloseHook(func(streamId uint32, peerId string, tags []string))`** — a
  post-removal callback the engine uses to reconcile trie interest on stream close, keyed by
  `streamId`.
- **`WithMetric(m, prefix)`** — registers namespaced gauges (the pubsub pool uses prefix
  `"pubsub"`), deliberately *not* the shared single-slot sync metric.
- **`RemoveTagsById(streamId, tags...)`** — removes tags from a specific stream, used by
  member eviction ([§7.4](#74-confidentiality--membership-change)) to stop delivery while the stream stays open.
- **`Broadcast` cross-tag dedup** — when handed more than one tag, `Broadcast` builds a `seen`
  set so a stream matching multiple patterns receives one copy.

---

## 12. Tests

The engine ships with a multi-peer in-memory fixture (in the synctest style) covering fan-out,
ACL rejection, relay rules, drop-on-overflow, dedup, echo suppression, ownership, and the
lifecycle edge cases. Notable regression tests:

- `TestReconnectKeepsInterest`, `TestStreamCloseDrainsInterest` — interest keyed by streamId.
- `TestResyncRestoresDeliveryAfterDrop` — resync loop revives a dropped subscriber.
- `TestCloseSpaceDropsInterest`, `TestEvictMemberStopsDelivery`,
  `TestRevalidateMembersEvictsNonMembers` — teardown and active eviction.
- `TestPubSubTopicOwnership`, `TestPubSubEchoSuppression` — `acc/` ownership at all layers,
  echo self-suppression.

Plus focused unit tests: `trie_test.go` (wildcard matching), `dedup_test.go` (ring),
`sign_test.go` (signature format), `topic_test.go` (validation).

---

## 13. Implementation status

**Done (this repo, `commonspace/pubsub` + `net/streampool`):** full wire protocol, flat +
wildcard topic model with the NATS-sublist trie, ACL-gated subscribe/publish, signed &
encrypted payloads, node relay with the one-hop rule, LAN symmetry, echo/duplicate-path
suppression, per-peer publish rate limiting, receive-path filter ordering + timestamp
staleness window + empty-identity guard, streamId-keyed interest with reconnect resync, and
active member eviction (`EvictMember` / `RevalidateMembers`).

**Deferred (documented, not silent):**

- **Zombie shedding** — close a stream stuck in queue-overflow for a sustained window. Needs
  per-stream drop stats from the pool; the drop-on-overflow bound already holds without it.
- **Subscribe-rate limiting / per-space lock sharding.** Serving-side interest is guarded by a
  single global mutex; a member can thrash subscribe/unsubscribe within the caps. Fine at
  expected scale; shard or rate-limit if a multi-tenant relay shows contention.
- **Reader-level payload cap.** The 64 KiB cap is enforced after DRPC decode; enforcing it at
  the stream reader (smaller buffer) needs per-stream buffer plumbing.
- **Rate-limiter size cap.** Per-peer buckets are time-GC'd but uncapped; bounded by
  authenticated members in practice.
- **Pubsub-specific metrics** — per-stream drop counter, slow-subscriber flag, one event per
  overflow episode, close-reason strings. The private pool is already observable via
  `Deps.Metric`; these are the additional counters. Per-topic counters are deliberately not
  added (unbounded cardinality); per-space is safe.
- **`MembershipChecker` returning a permission level** rather than a bare member/not — a
  one-way door kept simple for v1 (current-head `!NoPermissions()`).
- **`Close` blocks on in-flight dispatch handlers** with no timeout; a handler that violates
  the "must not block" contract wedges shutdown. Acceptable given the contract.

**Downstream wiring (separate repos, not in this repo):**

- **any-sync-node** — a `pubsubrelay` component: register `PubSub` next to SpaceSync via
  `RegisterRpc`; wire `Relay`/`Membership` from nodeconf + the hosted space's ACL; call
  `RevalidateMembers` (or `EvictMember`) from the ACL-update hook.
- **anytype-heart** — the client component: wire `Crypto` from the space ReadKey, `Peers`
  from the per-space peer manager, `SyncInterest` on `rebuildResponsiblePeers`, `CloseSpace`
  on space unload; serve inbound LAN pubsub streams from the existing client server; surface
  to apps via middleware commands (`PubsubPublish`, `PubsubSubscribe`/`Unsubscribe`) emitting
  `pb.Event`s through the local event bus. This side gets its own design doc.

**Compatibility & rollout.** Old peers don't know the `PubSub` service → DRPC unknown-RPC
error on stream open; the client treats the peer as "pubsub unavailable", backs off, and
retries opportunistically. No protoVersion bump, no coordinator/nodeconf changes. Ship order:
any-sync (this lib) → any-sync-node deploy → heart. Until nodes deploy, LAN-only pubsub still
works between updated clients.

---

## 14. Non-goals (v1)

Queue groups (1-of-N); persistence, replay, retained messages, catch-up after reconnect;
delivery receipts/acks; cross-space topics (patterns never span spaces — `spaceId` is a
separate field, not a topic segment); protocol-level presence; WAN client↔client (only
LAN-discovered direct peers); interest propagation between nodes (a node always forwards
client publishes to the other responsible nodes, which drop them if nothing matches locally —
2 bounded copies beats holding cross-node subscription state).

On that last non-goal, the NATS prior art maps cleanly onto our choice. NATS *clusters*
propagate interest (refcounted per subject) because a cluster may span many servers and
unnecessary fan-out is expensive at that scale — at the cost of every server holding the full
cluster interest map and an inherent propagation race. NATS *gateways* (WAN) instead default
to **optimistic sends**: forward without interest knowledge, let the receiver reply "no
interest", and switch to interest-only mode only after ~1000 rejections per account. With a
fixed fan-out of 2 peer nodes per space and shared-space traffic being likely-relevant to all
replicas, our always-forward is the optimistic-send strategy at the scale where it wins, and
it sidesteps NATS's biggest documented scaling pain (interest churn = a cluster-wide broadcast
plus a global cache flush per first-subscribe/last-unsubscribe). If inter-node waste ever
becomes measurable, the proven incremental fix is the gateway one: a bounded per-topic
no-interest map with a switch-to-interest-only threshold — not full interest replication.

---

## Appendix A — presence as an app pattern (non-normative)

Presence is deliberately **not** in the protocol. This is the recommended app recipe, modeled
on Yjs awareness with corrections where its semantics depend on a trusted relay. The critical
difference: y-websocket presence relies on the *server synthesizing* `state:null` for a dead
connection's clients. Our relay cannot forge signed messages, so that path does not exist
here — TTL expiry is the normative leave mechanism.

**Entry & lifecycle.** Each device publishes its full presence entry
`{sessionId, clock, state|null}` on state change and as a heartbeat every ~15 s (TTL/2);
receivers expire an entry after ~30 s (TTL) without re-announce, measured by
**receiver-local receipt time** (immune to sender clock skew). Full state per message, no
diffs: every message stands alone, so any at-most-once loss self-heals within one heartbeat,
and per-message signing composes cleanly.

- **TTL expiry is the normative leave.** Explicit leave (`state=null`) is a best-effort
  latency optimization on graceful shutdown. Worst-case ghost duration = TTL. (Matrix
  `m.typing` runs entirely on refresh-or-expire; that is the baseline.)
- **Key = `(accountId, sessionId)`, fresh random `sessionId` per app session.** An account
  with two devices is two entries; a rebooted client never needs to out-clock its dead
  predecessor — the old entry just times out. Never persist clocks across sessions.
  `accountId` comes from the verified message `identity`, never from the payload — signing
  closes the identity-hijack hole Yjs punts on.
- **Clock: bump before every publish** — state changes, heartbeats, leaves, reconnect
  re-announces alike, starting at 1. (Starting at 1 avoids the Yjs clock-0 trap where an
  unknown client's first entry is dead on arrival.)
- **Accept rule:** accept iff `clock > knownClock`, or (`clock == knownClock` and
  `state == null` and a live state exists) — equal-clock-null keeps leave idempotent under
  duplicate/multi-path delivery. On removal, keep a `sessionId → lastClock` tombstone for
  ≥ TTL so reordered pre-leave messages can't resurrect a ghost.
- **Join snapshot:** a stateless relay cannot push room state to a new joiner. Recipe:
  subscribe first, then announce yourself; peers treat an unknown-session announce as a cue to
  re-announce early (with random jitter ≤ heartbeat/2 to avoid an answer storm). Alternatively
  accept ≤ one heartbeat of join blindness.
- **Fan-out hygiene:** never re-publish an applied remote update (Yjs's origin-blind echo
  handler caused a documented N² storm at ~20 users/room); keep state small (identity +
  cursor); throttle high-frequency fields app-side (~10 Hz cursor max, further coalesced by
  the publish token bucket); consider separate topics and cadences for slow presence vs fast
  cursors. Idle-room budget is ≈ N²/heartbeat deliveries — scale the heartbeat up for large
  spaces.

Two topic layouts, both valid:

- **Shared topic** `presence` (or `presence/{objectId}`): one subscription covers the whole
  space; attribution comes from the verified `identity`.
- **Self-owned topics** `acc/online/<accId>`: spoof-proof per-account channels ([§7.2](#72-access-control)).
  Subscribe with `acc/online/*` to follow everyone at the cost of a single interest entry, or
  with concrete topics to follow specific accounts.

If sub-TTL leave latency ever matters, a v2 option is relay-emitted **unsigned transport
hints** ("subscriber stream closed") that clients may use only to shorten their local expiry
check for that peer — never as an authoritative removal; authority stays with signed messages
and TTL.
