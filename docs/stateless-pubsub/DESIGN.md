# Space-Scoped Stateless Pub/Sub over any-sync — Design

Status: **DESIGN / SPEC** — resolves all open tensions from [RESEARCH.md](./RESEARCH.md).
Grounded against `any-sync@main`, `any-sync-node@main`, `anytype-heart@main` (July 2026).

---

## 1. Summary

A new ephemeral, fire-and-forget, at-most-once publish/subscribe channel scoped to a
space, carried over a **dedicated DRPC bidi stream** fully isolated from the sync engine.
Topics are plaintext `/`-separated hierarchies inside a space; subscriptions may use
NATS-style wildcards (`*` one segment, `>` trailing segments). Any space member
(Reader/Guest included) may publish and subscribe. Payloads are end-to-end encrypted with the space ReadKey and
signed with the sender's account key; relay nodes route ciphertext they cannot read.
Fan-out goes through the responsible sync nodes **and** directly to LAN-discovered peers,
with a small bounded msgId dedup cache on receivers. No message is ever persisted; the
only routing state is transient in-memory interest state (stream tags + a pattern trie)
that dies with the connection.

### Resolved decisions

| Question (RESEARCH.md §7) | Decision |
|---|---|
| Meaning of "stateless" | Sense (a): no message persistence anywhere. Transient in-memory interest tables (stream tags) allowed and used. |
| Stream reuse vs dedicated | Dedicated `PubSub` DRPC service + own stream; never touches `ObjectSyncStream` or the sync dispatch path (user steer, RESEARCH.md §4.4). |
| New service vs new RPC on SpaceSync | New service, own proto package — independent versioning, unknown-service fallback for old peers. |
| Topic model | `/`-separated segment hierarchy within a space. Subscriptions may use NATS-style wildcards: `*` matches exactly one segment, `>` matches one-or-more trailing segments (tail-only). Publishers must use fully-qualified topics. Matching runs on a bounded per-space pattern trie at the relay (§4.1). |
| Topic privacy | Plaintext to relays. Comparable exposure to today's plaintext `objectId`s on the sync path. |
| Subscribe permission | Any space member: `!NoPermissions()` (`commonspace/object/acl/list/models.go:90`). |
| Publish permission | Any space member. Attribution via per-message account-key signature; abuse contained by per-peer rate limits. Plus a reserved **self-owned namespace**: topics `acc/…/<accId>` accept publishes only from `accId` (§6.2). |
| Payload confidentiality | Encrypted client-side with current space ReadKey + `keyId` indirection (push-server pattern). Removed member loses access at next key rotation. |
| Authenticity | Per-message account-key signature, verified by receivers. Protects against a semi-trusted node forging/reattributing messages. |
| Topology | Publisher → its responsible node (+ direct LAN peers). Node relays once to the other responsible nodes (`relayed` flag, never re-forwarded → loop-free). |
| Duplicate suppression | Receiver-side bounded FIFO ring keyed by `msgId` (duplicate paths exist by design: LAN + node). |
| Backpressure | Bounded queues, `TryAdd` drop-on-overflow end to end — the existing streampool discipline. Slow subscriber ⇒ dropped messages, never memory growth. |
| Queue groups (1-of-N) | Non-goal v1. |
| Catch-up / replay / retained messages | Non-goal, by definition of stateless. Reconnect ⇒ resubscribe ⇒ only new messages. |
| Presence lifecycle | Not in the protocol. v1 is a generic app API; presence (heartbeat/timeout/leave, Yjs-awareness style) is an app pattern on top (Appendix A). |

### Requirements compliance (RESEARCH.md §8)

- **R1 (on-protocol):** rides existing transports, secureservice handshake, DRPC mux, and a
  second streampool instance. A dedicated sub-stream on the existing `MultiConn` — no new
  transport or TLS handshake (`net/peer/peer.go:134`, `net/transport/transport.go:40-64`).
- **R2 (memory-effective):** per-stream bounded `mb.MB` queues with drop-on-overflow
  (`net/streampool/stream.go:32-44`), interest state = stream tags + pattern trie, both
  O(active subscriptions), fixed-size dedup FIFO ring, per-peer publish token bucket, caps on
  pattern count and topic/payload size. No path grows with message volume or offline
  duration.
- **R3 (ACL-aware):** node gates subscribe *and* publish on space membership at the current
  ACL head (a check that does **not** exist today on the sync path — this is new, additive
  enforcement); confidentiality holds against the relay via ReadKey encryption; membership
  removal cuts new traffic at key rotation and actively drops subscriptions (§6.4).

---

## 2. Semantics contract

- **At-most-once.** A publish reaching the relay is copied into bounded per-subscriber
  queues; overflow drops. No acks, no retransmit, no ordering across publishers, no dedup
  beyond the duplicate-path ring.
- **Fire-and-forget.** A publish with no subscribers is discarded. Nothing is stored.
- **Decoupled.** Publishers don't know subscribers. Subscriber set is whoever holds a live
  tagged stream at the instant of fan-out.
- **Subscribe is unacknowledged.** Success is silent (§3); interest takes effect when the
  serving peer processes the frame, so messages published concurrently by others may be
  missed. There is no "subscribed as of time T" guarantee — the same race NATS documents
  for cluster-wide subscription visibility (nats-server#1142), inherent to at-most-once.
  Apps needing a consistent starting state combine pub/sub with a snapshot read (e.g.
  presence: subscribe, then announce yourself, which prompts others' next heartbeat).
- **Reconnect = clean slate.** Subscriptions die with the stream; the client re-subscribes
  on reconnect and sees only new traffic.
- **Echo.** A publisher whose own interest set matches the topic receives its own message
  back from the relay; the client suppresses these by pre-recording its own msgId in the
  dedup ring at publish time, and delivers to local handlers via the normal dispatch queue
  (bounded, drop-on-overflow — so local delivery is best-effort like everything else, not
  a hard guarantee). Net effect = NATS `no_echo` semantics without a wire flag. (NATS
  echoes by default with per-connection opt-out; we don't need the option because dedup
  already exists.)
- **Delivery is best-effort, duplicates possible in theory** (ring eviction under extreme
  rates), so payload design must be idempotent/last-write-wins at the app level. In
  practice the ring makes duplicates vanishingly rare.

---

## 3. Wire protocol

New proto package in any-sync: `commonspace/pubsub/pubsubproto/protos/pubsub.proto`
(generated via the existing Makefile pipeline, `Makefile:23-32`).

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
  string topic           = 2;
  bytes  msgId           = 3;  // 16 random bytes, generated by publisher (dedup key)
  string keyId           = 4;  // space ReadKey id used for payload; "" = plaintext (keyless spaces)
  bytes  payload         = 5;  // ciphertext (or plaintext iff keyId == "")
  bytes  identity        = 6;  // sender account pubkey (marshalled)
  bytes  signature       = 7;  // account-key sig, see §6.3
  int64  timestampMilli  = 8;  // sender wall clock, informational
  bool   relayed         = 9;  // set by a node when forwarding node→node; never re-forwarded; excluded from signature
}

// Sent by the serving peer on rejected subscribe/publish. Success is silent.
message Status {
  string spaceId = 1;
  repeated string topics = 2;   // echo of the offending request (topic of a publish goes here too)
  ErrCodes code = 3;
}

enum ErrCodes {
  Ok = 0;
  NotAMember = 1;          // identity has no permissions in the space's ACL
  NotResponsible = 2;      // this node is not responsible for the space
  RateLimited = 3;
  TooManyTopics = 4;
  InvalidMessage = 5;      // malformed frame, oversized payload, identity mismatch, bad signature
  TopicNotOwned = 6;       // publish into acc/…/<accId> by an identity other than accId
  InvalidTopic = 7;        // malformed topic/pattern: bad wildcard placement, reserved chars, non-canonical form
  ErrorOffset = 800;       // 100..700 are taken by existing proto packages
}
```

Constraints (enforced by the serving peer, values are config defaults — §9). A rejected
Subscribe/Publish gets a `Status` and is otherwise ignored — the stream **stays open**
(the NATS precedent: `maximum subscriptions exceeded` is an error reply, not a
disconnect):

- topic: UTF-8, 1..256 bytes, `/`-separated segments (≤16 segments, no empty segments);
  recommended segment charset alnum + `.-_` (matches the NATS guideline of ≤16 tokens /
  ≤256 chars). Canonical form has **no leading `/`**
  (`acc/x` and `/acc/x` would silently be different topics — rejected as `InvalidTopic`).
- **Wildcards (subscription patterns only):** `*` matches exactly one segment
  (`chat/*/typing` ⇒ `chat/abc/typing`, not `chat/typing` or `chat/a/b/typing`);
  `>` matches one-or-more trailing segments and is only valid as the final segment
  (`chat/>` ⇒ everything under `chat/`). Wildcards must be complete segments (`chat/ty*`
  is invalid). `*` and `>` are reserved characters everywhere else; a `Publish` topic
  containing either is rejected — publishers always use fully-qualified topics.
- **Reserved self-owned namespace:** a topic whose first segment is `acc` (i.e. prefix
  `acc/`) is publishable **only** by the account whose id equals the topic's *last*
  segment — e.g. `acc/online/<accId>`, `acc/cursor/<accId>`. Subscribe remains open to
  all members (including patterns such as `acc/online/*`). Violations ⇒
  `Status{TopicNotOwned}`.
- payload: ≤ 64 KiB.
- per-stream interest set: ≤ 100 patterns per space, ≤ 1000 total.
- `identity` in a `Publish` **must equal** the connection-context identity
  (`net/peer/context.go:88`) when arriving from a client (`relayed == false`).
  This binds attribution to the TLS/handshake-proven account without requiring the node
  to verify the signature per message.

Interest tag format inside the pool: `spaceId + "/" + pattern`, verbatim (spaceId
contains no `/`; first separator wins). Wildcard resolution happens in the pubsub
engine's matcher, not in the pool — see §4.1.

---

## 4. Topology & relay rules

```
  publisher client ──publish──▶ responsible node A ──relayed=true──▶ node B ──▶ its subscribers
        │                            │                              node C ──▶ its subscribers
        │                            └──▶ A's local subscribers (tag fan-out)
        └──────direct publish──▶ LAN peers (same space, discovered via mDNS)
```

Relay rules (complete):

1. **Clients never forward.** A client receiving a `Publish` (from a node or a LAN peer)
   delivers it locally only.
2. **A node forwards only client-originated messages** (`relayed == false` arriving on a
   client stream): it stamps `relayed = true` and sends one copy to each *other*
   responsible node for the space (same peer-resolution logic as
   `any-sync-node/nodespace/peermanager/manager.go:87-103`), plus fans out to its local
   subscribers via the tag index.
3. **A node never forwards `relayed == true`** — it only fans out locally. With
   `ReplicationFactor = 3` (`nodeconf/nodeconf.go`), every message traverses at most
   client → node → node, hop limit 2, loop-free without any dedup state on nodes.
   (This is exactly the NATS cluster rule: "messages received from a route will only be
   distributed to local clients" — a strict one-hop limit is how NATS full-mesh clusters
   stay loop-free too.)
4. **A node rejects subscribe/publish for spaces it is not responsible for**
   (`Status{NotResponsible}`) — mirrors `checkResponsible`
   (`any-sync-node/nodespace/checks.go:14-24`).
5. **LAN peers are symmetric.** anytype-heart already runs a DRPC server for LAN peers
   (`space/spacecore/rpchandler.go`) and unifies node + LAN peers in the per-space peer
   manager (`space/spacecore/peermanager/manager.go:172-236`). Both sides run the same
   pubsub component; a LAN peer's stream carries Subscribe frames like a node's does, and
   publishes to LAN peers go direct. Clients apply the member-check on LAN subscribes the
   same way nodes do (they hold the ACL).

Duplicate paths are expected (a subscriber may get the same message from a LAN peer and
from its node). The **receiver** suppresses via a fixed-size FIFO ring keyed by `msgId`
(default 4096 entries ≈ 64 KiB of ids). Publishers self-suppress echoes by msgId too
(§2, Echo).

Client → node selection piggybacks on the existing responsible-peer choice
(`pool.GetOneOf(nodeIds)` — one node at a time, `manager.go:213`), so the pubsub stream
goes to the same node the client already syncs with.

### 4.1 Interest matching (wildcards)

The streampool tag index is exact-match (`streamIdsByTag`,
`net/streampool/streampool.go:76`), so the pubsub engine layers a matcher on top rather
than replacing the pool:

- Each accepted subscription registers its **pattern string verbatim as the stream tag**
  (`spaceId + "/" + pattern`) — the pool keeps doing stream bookkeeping (add/remove/GC
  on stream close) exactly as today.
- In parallel, the engine maintains a **per-space segment trie** of live patterns,
  mirroring the NATS sublist shape exactly (`server/sublist.go`): one level per segment,
  `map[segment]*node` for literals plus two dedicated wildcard slots per level (`pwc`
  for `*`, `fwc` for `>`), each terminal holding a refcount of subscribing streams.
  Match order as in NATS `matchLevel`: at each level add `fwc` matches, branch through
  `pwc`, hash-lookup the literal.
- On publish to concrete topic `T`: walk the trie with `T`'s segments — branching on the
  literal edge, the `*` edge, and any terminal `>` edge — collecting every matching
  pattern (O(segments × matched branches), segments ≤16). Then fan out once per matched
  pattern tag, **deduplicating stream ids across patterns**: a stream subscribed to both
  `chat/>` and `chat/*/typing` must receive one copy, not two. *Implemented* by teaching
  `streampool.Broadcast` to dedup stream ids across the tag set it's given (it previously
  collected per-tag with no cross-tag dedup) — so the engine passes all matched pattern
  tags to one `Broadcast` call and the pool guarantees one copy per stream.
- Trie cleanup: refcount decrement on Unsubscribe; on stream close the engine reconciles
  lazily — when a matched pattern's tag resolves to zero live streams, the pattern is
  dropped from the trie. (Alternative: a stream-close callback from the pool; decided at
  implementation time, both are bounded.)
- Exact-topic subscriptions are just patterns without wildcard segments — one code path.

The trie is bounded by the same caps as the interest set (≤1000 patterns/stream,
≤100/space/stream), so relay memory stays O(active subscriptions), and matching cost is
paid only per publish within that space.

**Deliberately no match-result cache.** NATS fronts its sublist with a 1024-entry
literal-subject → result cache, and its own issue history (nats-server#710, #941: <0.5%
hit rates, lock contention, latency spikes under sub/unsub churn) led to `NoCache`
sublists — which NATS uses for exactly our analog, the small per-connection permission
tries. Our tries are per-space (small) and ephemeral interest is churny (every
subscribe/unsubscribe would invalidate), so a direct walk of a ≤16-level trie beats a
cache we'd constantly flush. Revisit only with profiling evidence; the bounded
patch-on-insert design from NATS is the template if ever needed.

---

## 5. What happens on each frame (serving peer)

**Subscribe** —
resolve space ACL state (nodes: the space is hosted locally; clients/LAN: the open space);
check `PermissionsAtRecord(head, ctxIdentity)` is not `None`; validate patterns
(`InvalidTopic` on bad wildcard placement / reserved chars / non-canonical form); check
pattern-count caps; then register in the trie and `AddTagsCtx(ctx, spaceId+"/"+pattern...)`.
Reject ⇒ `Status`, no tag.

**Unsubscribe** — `RemoveTagsCtx` + trie refcount decrement. No checks needed.

**Publish** (from client stream) —
1. size/shape checks (a topic containing `*`/`>` ⇒ `InvalidTopic`);
   `identity == ctxIdentity`; membership check against cached ACL state (map lookup);
   self-owned-namespace check (topic `acc/…` ⇒ last segment must equal
   `ctxIdentity.Account()` — one string compare); per-peer token bucket (§7).
2. Match the topic against the space's pattern trie (§4.1), dedup stream ids across
   matched patterns, then `Broadcast(msg, matchedPatternTags...)` on the pubsub pool —
   the existing tag-index fan-out (`net/streampool/streampool.go:360-377`), which
   per-stream `TryAdd`s and drops on overflow.
3. If serving peer is a responsible node: stamp `relayed=true`, send to other responsible
   nodes (lazy stream open via the pool's `Send` + PeerGetter).

**Publish** (`relayed == true`, from a node stream) —
verify the sending peer is a responsible node for the space (peerId ∈ `NodeIds(spaceId)`);
fan out locally only (same trie match). The `identity == ctxIdentity` rule does not apply
(the forwarding node is not the author) — authenticity is the receiver's signature check
(§6.3).

**Receive** (client) — dedup by msgId ring → verify signature against `identity` → check
`identity` is a member at the local ACL head → if the topic is in the `acc/` namespace,
check its last segment equals `identity.Account()` (end-to-end enforcement: the signature
covers the topic, so not even a malicious relay can inject into someone else's self-owned
topic) → look up ReadKey by `keyId`, decrypt → dispatch to local topic handlers. Any
failure ⇒ drop + debug metric, never an error to the peer.

---

## 6. Security model (R3)

### 6.1 Threat model — the semi-trusted relay

The node can: observe spaceIds, topics, sender identities, timing, sizes (accepted —
comparable to sync-path metadata today); drop, delay, reorder messages (accepted —
at-most-once contract). The node cannot: read payloads (no ReadKey); forge or reattribute
messages (signature); replay effectively. Replay defense is two-layer: the msgId dedup
ring catches the short window, and the receiver enforces a **signed-timestamp staleness
window** (`Config.MaxTimestampSkew`, default 5 min) so that even after the ring evicts an
id, a relay replaying an old signed frame is rejected on its stale timestamp. (A relay
cannot forge a fresh timestamp — it's covered by the signature.)

### 6.2 Access control

Both directions gate on **space membership at the current ACL head**, checked by the
serving peer from its local ACL copy — a cached in-memory lookup, no coordinator round
trip. This is *new* enforcement: today's sync path has none at subscribe time
(`any-sync-node/nodespace/rpchandler.go:329-331` accepts unchecked). Publish and
subscribe both require `!NoPermissions()`; no `CanWrite` requirement (decision: any
member publishes — presence/typing-style uses need Readers to emit).

**Self-owned topics.** The `acc/` namespace (§3) adds a per-topic ownership rule on top
of membership: only the account named by the topic's last segment may publish there.
It is enforced twice — at the serving peer (cheap, because `identity == ctxIdentity` is
already bound by the handshake) and at every receiver (via the signature, which covers
the topic string). This gives apps spoof-proof per-account channels — e.g.
`acc/online/<accId>` — where consumers can trust the topic itself, not just the message
attribution. It is the in-protocol generalization of the push-server's "silent
self-channel" restriction (`anytype-push-server/push/push.go:140`), and matches the
proven NATS pattern for the same problem: per-identity subject prefixes (the
`_INBOX_<user>.>` convention) rather than dynamic per-message grants. NATS violation
semantics also match ours: a permissions violation drops the message / rejects the
subscribe with an error and keeps the connection open — only authentication failures
disconnect. Wildcards make the
fan-in side cheap: one `acc/online/*` subscription covers every member's online topic,
and each received message is still individually ownership-checked against its concrete
topic and verified signature.

### 6.3 Authenticity

`signature = accountKey.Sign("anysync:pubsub:v1" | len‖spaceId | len‖topic | len‖msgId |
len‖keyId | le64(timestampMilli) | payload)`. Each variable-length field is length-prefixed
(le32) so field boundaries can't shift (e.g. `spaceId="ab",topic="c"` vs
`spaceId="a",topic="bc"` sign differently); `payload` trails unprefixed as the final field.
`relayed` is excluded (mutated in transit). Receivers verify; nodes don't need to
(attribution from clients is already bound by `identity == ctxIdentity`, and verifying
per-message on the relay buys little at real CPU cost). Ed25519 sign/verify is ~30-80 µs —
negligible at ephemeral-signal rates.

Receivers run the cheap filters (local-interest match, membership, `acc/` ownership,
timestamp staleness) *before* the Ed25519 verify, and record the dedup ring only after
verify succeeds — so a relay/LAN-peer flood of forged-signature messages is shed cheaply
and cannot evict legitimate ids from the ring to reopen a replay window (§6.1).

### 6.4 Confidentiality & membership change

Payloads encrypt with `AclState.CurrentReadKey()` (`commonspace/object/acl/list/aclstate.go:170`),
carrying `CurrentReadKeyId()` as `keyId`. Receivers hold historical keys via the ACL, so
rotation mid-flight is safe. On member removal the existing rotation
(`aclrecordbuilder.go:761-844`) cuts decryption of new traffic automatically. Additionally,
the engine exposes `EvictMember(spaceId, identity)` — the node wires it to its ACL-update
hook (the `syncacl` updater, `commonspace/object/acl/syncacl/syncacl.go:53-56`, is the
precedent) to actively drop the removed identity's subscriptions: it strips that account's
per-stream tags (via `streampool.RemoveTagsById`, so delivery — which is tag-keyed — stops
even while the stream stays open) and decrements the match trie. Bounded work: one scan of
the streams subscribed to that space per ACL change.

Spaces without a ReadKey (post-GO-7187 keyless/public spaces): `keyId = ""`, plaintext
payload, signature still required.

---

## 7. Memory & abuse bounds (R2)

| Resource | Bound | Mechanism |
|---|---|---|
| Outbound per-stream buffer | `queueSize` msgs (default 100) | `mb.MB` + `TryAdd` drop (`net/streampool/stream.go:39`) |
| Interest table | ≤1000 patterns/stream, ≤100/space/stream | reject with `TooManyTopics`; state dies with stream |
| Pattern trie (§4.1) | O(total live patterns × segments), segments ≤16 | same caps as interest table; lazily pruned when a pattern's tag has no live streams |
| Publish rate | token bucket **per peer** (default 30 msg/s, burst 60) | checked in pubsub handler — **inside** the stream, because the RPC limiter only gates stream-open (`net/rpc/limiter/limiter.go:96-106`). One bucket per peer (all of a peer's streams share it — stricter than per-stream, so a peer can't multiply its budget by opening streams). Deliberate divergence from core NATS, which has no publish rate limiting (maintainers punt to network throughput) and instead disconnects slow consumers — acceptable for a trusted-client broker, not for our semi-trusted multi-tenant relays. We also drop rather than disconnect slow subscribers, which fits at-most-once ephemera |
| Payload size | ≤64 KiB | reject `InvalidMessage` |
| Dedup cache | fixed FIFO ring, 4096 msgIds | evicts oldest, O(1) |
| Fan-out amplification | 1 upload → ≤2 node-node copies → N subscriber queues | node-side copy (publisher uploads once); `peerMessage.Copy()` pattern reuses the existing per-destination stamping (`stream.go:32-37`) |
| Idle streams | closed with the sub-connection; tags GC'd in `removeStream` (`streampool.go:431-446`) | existing |
| Zombie subscribers | stream in continuous queue-overflow for > 30 s is closed | NATS disconnects slow consumers outright to protect the system; we drop first (fits at-most-once), but a *persistently* full queue means a dead/wedged reader burning fan-out work — shed it and let the client reconnect fresh |

No persistence, no unbounded map, no per-message allocation beyond the pooled message
structs (mirror `objectmessages` `sync.Pool` usage, `headupdate.go:13-38`).

---

## 8. Component design per repo

### 8.1 any-sync (this repo)

1. **Generalize streampool for a second instance.** Extract a non-component constructor —
   `streampool.NewPool(handler streamhandler.StreamHandler, cfg StreamConfig) Pool` — and
   make the existing component (`streampool.go:27`, hard-bound to `streamhandler.CName`
   at `streampool.go:98`) a thin wrapper. Backward compatible; the pubsub service embeds
   its own pool with its own handler, queues, and tags. Sync flow control is untouched.
2. **`commonspace/pubsub/pubsubproto`** — proto + generated DRPC (Makefile pipeline).
3. **`commonspace/pubsub`** — the shared engine used by node, client, and LAN-server
   sides alike:
   - stream lifecycle: `OpenStream` to a peer / `ReadStream` for inbound (both feed the
     private pool), resubscribe-on-reconnect using the `subscribeclient` watcher pattern
     (`coordinator/subscribeclient/client.go:115-153`);
   - interest handling: Subscribe/Unsubscribe → pattern validation + membership check
     hook → per-space pattern trie (§4.1) + tags;
   - publish path: validate → trie match + cross-pattern stream dedup → broadcast →
     optional forward hook;
   - receive path: dedup ring → verify → decrypt → handler dispatch;
   - pluggable interfaces so layering stays clean:
     `MembershipChecker` (backed by `AclState`), `Crypto` (ReadKey encrypt/decrypt via the
     space's `Acl()`), `Forwarder` (node-only), `RateLimiter`.
   - public API:
     `Publish(ctx, spaceId, topic string, payload []byte) error` (encrypt+sign+send) and
     `Subscribe(spaceId, topic string, h Handler) (unsubscribe func())`, plus an error/
     status callback for surfaced `Status` frames.
4. **Reference wiring + tests** in the synctest style
   (`commonspace/sync/synctest/`): multi-peer in-memory fixture proving fan-out, ACL
   rejection, relay rules, drop-on-overflow, dedup.

### 8.2 any-sync-node

- Register `DRPCRegisterPubSub` next to SpaceSync (`nodespace/service.go:80`).
- `pubsubrelay` component: wires the shared engine with node deps — membership from the
  hosted space's ACL, responsibility check from nodeconf, `Forwarder` resolving the other
  responsible nodes (reuse `getResponsiblePeers` logic,
  `nodespace/peermanager/manager.go:87-103`), rate-limiter config.
- ACL-update hook to evict removed members' tags (§6.4).

### 8.3 anytype-heart (sketch — own design doc when we get there)

- Register the pubsub client component in bootstrap next to streampool
  (`core/anytype/bootstrap.go:263-281`); open streams to the space's responsible node and
  LAN peers via the existing per-space peer manager; serve inbound LAN pubsub streams from
  the existing client server (`space/spacecore/rpchandler.go`).
- Tie subscriptions to space open/close lifecycle; auto-resubscribe on
  `rebuildResponsiblePeers`.
- Surface to apps: middleware commands (`PubsubPublish`, `PubsubSubscribe/Unsubscribe`)
  emitting `pb.Event`s through the existing local event bus (`core/subscription/`), the
  same delivery surface chat SSE uses.

### 8.4 Compatibility & rollout

- Old peers don't know the `PubSub` service → DRPC unknown-RPC error on stream open.
  Client treats it as "pubsub unavailable on this peer", backs off (subscribeclient's
  capped linear backoff), and retries opportunistically. No protoVersion bump required;
  no coordinator/nodeconf changes (same node addresses, same responsibility mapping).
- Ship order: any-sync (lib) → any-sync-node deploy → heart. Until nodes deploy, LAN-only
  pubsub still works between updated clients.

---

## 9. Defaults (tunable via config)

| Knob | Default |
|---|---|
| max payload | 64 KiB |
| max topic length | 256 B |
| max topics per stream | 1000 (100 per space) |
| publish rate per peer | 30 msg/s, burst 60 |
| per-stream write queue | 100 (client), 500 (node outbound) — match sync-side sizes |
| dedup ring | 4096 msgIds |
| received-timestamp staleness window | 5 min (enforced at receive, `Config.MaxTimestampSkew`) |
| client interest resync interval | 20 s (`Config.ResyncInterval`) |
| pubsub stream peer TTL | 1 h (`Config.PeerTTL`) |

## 10. Non-goals (v1)

Queue groups (1-of-N); persistence, replay, retained messages, catch-up after reconnect;
delivery receipts/acks; cross-space topics (patterns never span spaces — `spaceId` is a
separate field, not a topic segment); protocol-level presence; WAN client↔client (only
LAN-discovered direct peers); interest propagation between nodes (a node always forwards
client publishes to the other responsible nodes, which drop them if nothing matches
locally — 2 bounded copies beats holding cross-node subscription state).

On that last non-goal, NATS prior art maps cleanly onto our choice. NATS *clusters* do
propagate interest (RS+/RS-, refcounted per subject, advertised on the 0→1 transition,
withdrawn on N→0) because a cluster may span many servers and unnecessary fan-out is
expensive at that scale — the cost is every server holding the full cluster interest map
and an inherent propagation race (subscription visibility across the cluster is
asynchronous, nats-server#1142). NATS *gateways* (WAN) instead default to **optimistic
sends**: forward without interest knowledge, let the receiver reply "no interest", and
only switch to interest-only mode after ~1000 such rejections per account
(`server/gateway.go`, `defaultGatewayMaxRUnsubBeforeSwitch`). With a fixed fan-out of 2
peer nodes per space and shared-space traffic being likely-relevant to all replicas, our
always-forward is the optimistic-send strategy at the scale where it wins. It also
sidesteps NATS's single biggest documented scaling pain — interest churn, where every
first-subscribe/last-unsubscribe is a cluster-wide broadcast plus a global client-cache
flush (nats-server#710/#941). If inter-node waste ever becomes measurable, the proven
*incremental* fix is the gateway one: a bounded per-topic no-interest map with a
switch-to-interest-only threshold — not full RS+/RS- interest replication (v2, not v1).

## 11. Remaining open items

1. Exact package/service naming bikeshed (`commonspace/pubsub` vs top-level `pubsub`;
   service `PubSub` vs `SpacePubSub`).
2. Whether the node emits `Status{RateLimited}` per rejected publish or silently drops
   after the first notice (flood of statuses is itself amplification — leaning: notify
   once per window).
3. Metrics surface (per-topic counters are unbounded-cardinality; per-space is safe).
   The private pool is observable via `Deps.Metric` (`WithMetric(m, "pubsub")`), which
   registers namespaced prometheus gauges (stream/tag/dial counts). It deliberately does
   **not** register into the shared single-slot sync-metric (`RegisterStreamPoolSyncMetric`)
   — that belongs to the app-level sync streampool, and a second pool there would overwrite
   and, on Close, null it. The remaining work is the pubsub-specific counters below.

## 12. Implementation status (v1 in any-sync)

Implemented in `commonspace/pubsub` (+ `net/streampool` additions): full wire protocol,
flat+wildcard topic model with the NATS-sublist trie, ACL-gated subscribe/publish, signed
& encrypted payloads, node relay with the one-hop rule, LAN symmetry, echo/duplicate-path
suppression, per-peer publish rate limiting, and — after the multi-lens review — the
following hardening:

- **Interest keyed by streamId, not peerId.** Serving-side interest lives in
  `streams[streamId]` and the match trie refcounts *subscribing streams*; the close hook
  carries the `streamId`. This fixes two review-found bugs: a reconnecting peer's fresh
  stream no longer loses interest when the stale stream closes, and a stream that dies
  mid-subscribe can't orphan interest (interest+tag are committed under one lock with
  rollback if the stream vanished). Regression tests: `TestReconnectKeepsInterest`,
  `TestStreamCloseDrainsInterest`.
- **Reconnect watcher.** A resync loop re-pushes local interest every `ResyncInterval`
  and immediately on client-side stream close, and `OpenStream` sets `PeerTTL` so idle
  pool GC doesn't silently reap a quiet subscriber. Without this a pure subscriber went
  dead after the first drop. Test: `TestResyncRestoresDeliveryAfterDrop`.
- **`CloseSpace` / `EvictMember`.** Per-space teardown (a global service must release
  closed-space state) and §6.4 active member eviction. Tests: `TestCloseSpaceDropsInterest`,
  `TestEvictMemberStopsDelivery`.
- **Receive-path ordering + timestamp window + empty-identity guard** (§6.1/§6.3).
- **`Status.msgId`** for host-side rejection correlation; node publish shape errors return
  `InvalidTopic` consistently with the client API; `spaceId` is validated to contain no `/`.

Deferred (documented, not silent):

- **Zombie shedding** (close a stream stuck in queue-overflow > 30 s, §7). Needs per-stream
  drop stats surfaced from the pool; the drop-on-overflow bound already holds without it.
- **Subscribe-rate limiting / per-space lock sharding.** `remoteMu` is a single global
  lock; a member can thrash subscribe/unsubscribe within the caps. Fine at expected scale;
  shard or rate-limit if a multi-tenant relay shows contention.
- **Reader-level payload cap.** The 64 KiB cap is enforced after DRPC decode; enforcing it
  at the stream reader (smaller `MaximumBufferSize`) needs per-stream buffer plumbing.
- **Rate-limiter size cap.** Per-peer buckets are time-GC'd but uncapped; bounded by
  authenticated members in practice.
- **pubsub-specific metrics** (per-stream drop counter, slow-subscriber flag, one event per
  overflow episode, close-reason strings) — §11.3.
- **`MembershipChecker` returning a permission level** rather than a bare member/not — a
  one-way door kept simple for v1 (current-head `!NoPermissions()`).
- **Close blocks on in-flight dispatch handlers.** `Close` waits for the dispatch loop to
  drain; a handler that violates the "must not block" contract wedges shutdown (no timeout).
  Acceptable given the contract; a stuck app handler is an app bug, and adding a Close
  timeout would mask it.
- **Resync coverage for many-space clients.** Each resync pass fires one dial task per local
  space through the bounded dial queue (`DialQueueSize`, default 100); a client with more
  spaces than that drops the overflow for that tick and restores their interest on a later
  tick (self-correcting, no leak — re-sends are idempotent). Heart should size
  `DialQueueSize` to its expected open-space count.

Downstream wiring (separate repos, per §8.2/§8.3): any-sync-node `pubsubrelay` component
(`Relay`/`Membership` from nodeconf + hosted ACL, `RegisterRpc`, `EvictMember` on ACL
change) and anytype-heart client (`Crypto` from the space ReadKey, `PeerProvider` from the
peer manager, `CloseSpace` on space unload, middleware commands).
   Should follow the nats.go subscription observability contract: per-stream dropped
   counter, a "slow subscriber" state flag, **one event per overflow episode** (not per
   dropped message), plus a cumulative per-node counter (varz `slow_consumers` style)
   and distinct close-reason strings (rate-limited vs zombie-shed vs transport error).

---

## Appendix A — presence as an app pattern (non-normative recipe)

Modeled on Yjs awareness (`y-protocols/awareness.js`), with deliberate corrections where
its semantics depend on a trusted relay. The critical difference: **y-websocket presence
relies on the server synthesizing `state:null` for a dead connection's clients** —
current y-websocket clients send *no* leave on tab close at all (the unload handler was
deliberately removed, yjs/y-websocket#165). Our relay cannot forge signed messages, so
that path does not exist here.

**Entry & lifecycle.** Each device publishes its full presence entry
`{sessionId, clock, state|null}` on state change and as a heartbeat every ~15 s
(TTL/2); receivers expire an entry after ~30 s (TTL) without re-announce, measured by
**receiver-local receipt time** (immune to sender clock skew — Yjs's `lastUpdated` never
crosses the wire either). Full state per message, no diffs: every message stands alone,
so any at-most-once loss self-heals within one heartbeat, and per-message signing
composes cleanly.

- **TTL expiry is the normative leave mechanism.** Explicit leave (`state=null`) is a
  best-effort latency optimization on graceful shutdown. Worst-case ghost duration =
  TTL. (Matrix `m.typing` runs entirely on refresh-or-expire; that is the baseline
  guarantee.)
- **Key = `(accountId, sessionId)`, fresh random `sessionId` per app session** (Yjs:
  new `clientID` per page load). An account with two devices is two entries; a rebooted
  client never needs to out-clock its dead predecessor — the old entry just times out.
  Never persist clocks across sessions. `accountId` comes from the verified message
  `identity`, never from the payload — signing closes the identity-hijack hole Yjs
  explicitly punts on (`PROTOCOL.md §6`), and Yjs's own-clientID `clock++` defense
  becomes unnecessary.
- **Clock: bump before every publish** — state changes, heartbeats, leaves, and
  reconnect re-announces alike, starting at 1. (Yjs leaves some paths un-bumped only to
  interoperate with server-synthesized equal-clock nulls, and its un-bumped reconnect
  re-announce causes a real invisibility race; with no synthesizing relay, always-bump
  is strictly simpler and safer. Starting at 1 avoids the Yjs clock-0 trap where an
  unknown client's first entry is dead on arrival.)
- **Accept rule:** accept iff `clock > knownClock`, or (`clock == knownClock` and
  `state == null` and a live state exists) — equal-clock-null keeps leave idempotent
  under duplicate/multi-path delivery. On removal (null or TTL), keep a
  `sessionId → lastClock` tombstone for ≥ TTL so reordered pre-leave messages can't
  resurrect a ghost.
- **Join snapshot:** a stateless relay cannot push the room state to a new joiner
  (y-websocket's server does). Recipe: subscribe first, then announce yourself; peers
  treat an unknown-session announce as a cue to re-announce early (with random jitter
  ≤ heartbeat/2 to avoid an answer storm). Alternatively accept ≤ one heartbeat of join
  blindness.
- **Fan-out hygiene:** never re-publish an applied remote update (Yjs's origin-blind
  echo handler caused a documented N² message storm at ~20 users/room); keep state
  small (identity + cursor); throttle high-frequency fields app-side (~10 Hz cursor
  max, further coalesced by the publish token bucket); consider separate topics and
  cadences for slow presence vs fast cursors. Idle-room budget is ≈ N²/heartbeat
  deliveries — scale the heartbeat up for large spaces.

Two topic layouts, both valid:

- **Shared topic** `presence` (or `presence/{objectId}`): one subscription covers the
  whole space; attribution comes from the verified `identity`.
- **Self-owned topics** `acc/online/<accId>`: spoof-proof per-account channels (§6.2).
  Subscribe with `acc/online/*` to follow everyone at the cost of a single interest
  entry, or with concrete topics to follow specific accounts.

If sub-TTL leave latency ever matters, a v2 option is relay-emitted **unsigned transport
hints** ("subscriber stream closed") that clients may use only to shorten their local
expiry check for that peer — never as an authoritative removal; authority stays with
signed messages and TTL.
