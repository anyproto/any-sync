# Stateless Pub/Sub in `any-sync` — Research Summary

Status: **RESEARCH / PROBLEM-FRAMING** — deliberately contains **no chosen solution**.
Purpose: hand-off document for a spec author (fable). It frames the problem, surveys how
NATS and other modern systems do stateless pub/sub, maps the concrete `any-sync` substrate
(`file:line`-grounded against `main`), catalogs the prior art already in the repo, and
enumerates the open design tensions the spec must resolve. It does **not** pick a design.

---

## 0. The ask (verbatim intent)

> Users of the same space can **subscribe/publish topics within a space**. It must work
> **effectively over the any-sync protocol** (probably a DRPC stream), be **memory-effective**,
> and **respect the ACL access** of the user.

Three hard requirements fall out of this, and every section below is oriented around them:

1. **R1 — On-protocol.** Runs over the existing any-sync transport/DRPC machinery, not a side channel.
2. **R2 — Memory-effective.** Bounded, ephemeral footprint on both clients and relaying nodes; no unbounded buffers, no persistence.
3. **R3 — ACL-aware.** Publish/subscribe is gated by the space's access-control list and its encryption boundary.

Plus the implicit scope word: **stateless** (Section 1 pins down what that actually means — it is ambiguous and the ambiguity matters).

---

## 1. What "stateless pub/sub" means (and the ambiguity to resolve)

Distilled from the reference systems (Sections 2–3), "stateless pub/sub" is the **core-NATS / Redis-pub-sub** family, characterized by:

- **Fire-and-forget.** A publish with no live subscriber is simply discarded; it is never stored or replayed.
- **At-most-once delivery** (MQTT "QoS 0"). No acks, no retransmit, no dedup, no ordering guarantees across publishers.
- **No persistence.** No log, no durable queue, no retained "last message." (This is exactly what distinguishes it from any-sync's DAG trees, the KV store, and the coordinator Inbox — all of which *are* stateful.)
- **Full decoupling** of publishers and subscribers in space (don't know each other), time (needn't overlap beyond the instant of delivery), and synchronization (non-blocking).
- **Fan-out (1→N)** as the base pattern; optionally **load-balanced 1-of-N** ("queue groups").

**Ambiguity the spec MUST pin down — two independent axes of "stateless":**

- **(a) Message statelessness** — no message is ever persisted (fire-and-forget). *All* systems in this family have this.
- **(b) Subscription/routing statelessness** — whether the *relay* holds any in-memory subscription-interest table.
  - Core NATS is stateless in sense (a) but the **server does hold an in-memory interest graph** (subject → subscribers) to route efficiently. It is *not* stateless in sense (b).
  - The fully-(b)-stateless alternative is "relay broadcasts everything, subscribers filter locally" — zero routing state but poor bandwidth-efficiency.

This tension between (b) and **R2 (memory-effective)** / bandwidth-efficiency is one of the central design decisions and is called out again in Section 7. The word "stateless" in the ask most plausibly means (a) + *no durable/persistent* state, tolerating transient in-memory routing tables — but this must be confirmed, not assumed.

---

## 2. Reference model: core NATS pub/sub

(Sources: NATS docs — pubsub, subjects, queue groups; see Section 9.)

**Subject-based addressing.** Publishers send to a **subject** (a string); subscribers register **interest** in subjects. "A subject is just a string the publisher and subscriber use to find each other." Messages carry `{subject, payload bytes, headers, optional reply-address}`.

**Subject hierarchy + wildcards.**
- Subjects are dot-separated token hierarchies: `time.us.east.atlanta`.
- **`*`** matches exactly one token: `time.*.east` ⇒ `time.us.east`, `time.eu.east` (not `time.us.east.atlanta`).
- **`>`** matches one-or-more trailing tokens, tail-position only: `time.us.>` ⇒ everything under `time.us`.
- **Publishers must use fully-qualified subjects** (no wildcards); **only subscribers use wildcards.**
- Allowed chars: any Unicode except null, space, `.`, `*`, `>`; recommend alnum + `-`/`_`. `$`-prefixed reserved for system. Guideline ≤16 tokens, ≤256 chars. Max payload default 1 MB (server `max_payload`, cap 64 MB).

**Delivery.**
- **Fan-out:** every interested subscriber gets a copy (1→N).
- **Queue groups:** subscribers sharing a queue name form a group; each message goes to **exactly one randomly-chosen** member — built-in load balancing + transparent scaling + "no-responders" signal. Queue-group names follow subject naming rules.
- **At-most-once.** Offline/disconnected subscriber ⇒ message lost. Messages with no subscribers are discarded.

**What core NATS deliberately does NOT provide** (these are JetStream, a separate stateful layer): persistence, replay, guaranteed/at-least-once delivery, dedup, ordering, consumer cursors. Those are exactly the things a *stateless* design excludes.

---

## 3. Landscape of modern pub/sub (comparison)

| System | Topic model | Wildcards | Delivery | Persistence | Topology | Notable for us |
|---|---|---|---|---|---|---|
| **NATS core** | dot-hierarchy subjects | `*` (1 token), `>` (tail multi) | fan-out + queue-group (1-of-N) | none (JetStream is separate) | central broker(s), interest graph | the canonical model; subject wildcards; queue groups |
| **Redis pub/sub** | flat channels + patterns | `*`, `?`, `[..]` (glob) | fan-out | none | central server | simplest fire-and-forget; subscriber must be connected |
| **MQTT** | `/`-hierarchy topics | `+` (1 level), `#` (tail multi) | QoS 0/1/2 | retained msg + sessions ⇒ **stateful** | broker | wildcard syntax variant; "retained message" is the anti-pattern to avoid for stateless |
| **libp2p GossipSub** | flat topics | none | epidemic mesh fan-out; IHAVE/IWANT lazy-pull | none | **decentralized P2P mesh** (no broker) | closest to any-sync's decentralization ethos; mesh + fanout + peer-scoring for abuse resistance |
| **Yjs awareness** | one implicit channel/doc | n/a | state-based CRDT diff | ephemeral, auto-GC | transport-agnostic relay | **presence prototype**: `(clientID, clock, state\|null)`, `null`=offline, 15 s re-announce / 30 s timeout; kept *separate* from the CRDT doc |
| **Phoenix Channels** | `topic:subtopic` strings | none (exact) | fan-out; Presence built on PubSub | ephemeral | server (BEAM PubSub) | presence = minimal ephemeral metadata over pub/sub |
| **Matrix EDUs** | per-room | n/a | fan-out to room servers | ephemeral (EDU ≠ persistent event) | federated servers | typing/presence modeled as **Ephemeral Data Units**, explicitly distinct from the persistent event DAG |

**Cross-cutting takeaways relevant to any-sync:**

- **Topology is the fork in the road.** NATS/Redis/MQTT = central broker with an interest table. GossipSub = P2P mesh, no broker. any-sync sits *between*: clients reach a small set of **responsible sync nodes** (semi-trusted relays that store ciphertext they cannot read) and *may* also reach other clients directly (Section 4.6). The relay-through-node model is closest to a broker, but the broker is **untrusted for content** — which forces the encryption question (Section 6).
- **Presence is the killer stateless-pubsub use case** in local-first / collaborative systems (Yjs awareness, Phoenix Presence, Matrix EDUs), and it is *always* kept separate from the persistent CRDT/document layer. If presence/typing/cursors are a target use case, the Yjs awareness lifecycle (heartbeat + timeout + `null`-on-leave) is the reference to study, not NATS.
- **Wildcards cost the relay.** Subject-hierarchy matching (`*`/`>`, `+`/`#`) requires the relay to run a trie/interest-match per message. Flat topics (GossipSub, Redis channels, Phoenix) do not. This trades expressiveness against R2.

---

## 4. The `any-sync` substrate (grounded map)

All paths under `/Users/roma/anytype/any-sync`, line numbers vs `main`. **Scope caveat:** any-sync is a *library*. The concrete server-side `DRPCSpaceSyncServer` and the production `PeerManager`/`StreamHandler` live in downstream repos (`any-sync-node`, `anytype-heart`). This repo ships the interfaces, the client plumbing, and **reference implementations in test code** (`commonspace/spaceutils_test.go`, `commonspace/spacerpc_test.go`, `commonspace/sync/synctest/`). Every such boundary is flagged.

### 4.1 Transport & DRPC (R1 foundation)

- Three transports selected by address scheme: **Yamux** (TCP), **QUIC**, **WebTransport** — `net/transport/transport.go:24-28`. ALPN `"anysync"`; QUIC handshake at `net/transport/quic/quic.go:108-165`.
- Secure layer = libp2p-TLS + an app handshake that stamps `peerId`, account `identity`, versions into the connection context — `net/secureservice/secureservice.go:114-185`, ctx accessors `net/peer/context.go:33-106`.
- DRPC server is a `drpcmux` with a handler chain `limiter → metric → encoding` — `net/rpc/server/drpcserver.go:52-80`. Services register via generated `DRPCRegisterXxx(mux, impl)`.
- A bidirectional DRPC stream is obtained by `peer.AcquireDrpcConn` → generated stream method → `drpc.Stream{Send,Recv,MsgSend,MsgRecv}` — `net/peer/peer.go:134,270-297`.

### 4.2 StreamPool — the fan-out core (R1 + R2)

`net/streampool/streampool.go:51-69` — the central abstraction. It caches opened `drpc.Stream`s, **indexes them by peer and by tag**, opens them lazily, and pushes messages onto per-stream write queues:

```go
type StreamPool interface {
    app.ComponentRunnable
    AddStream(stream drpc.Stream, queueSize int, tags ...string) error   // outgoing
    ReadStream(stream drpc.Stream, queueSize int, tags ...string) error  // incoming, blocks reading
    Send(ctx, msg drpc.Message, target PeerGetter) error                 // dial+send, async
    SendById(ctx, msg drpc.Message, peerIds ...string) error             // only if stream exists
    Broadcast(ctx, msg drpc.Message, tags ...string) error               // fan-out to all streams with tag
    AddTagsCtx(ctx, tags ...string) error                                // subscribe a stream to tag(s)
    RemoveTagsCtx(ctx, tags ...string) error                             // unsubscribe
    Streams(tags ...string) []drpc.Stream
}
```

- Tag index: `streamIdsByTag map[string][]uint32` — `streampool.go:71-84`. In practice the tag is a **`spaceId`** (see 4.4).
- `Broadcast(msg, tags...)` writes to every stream carrying a listed tag — `streampool.go:360-377`. **This is the existing tag-keyed fan-out primitive.**
- `AddTagsCtx`/`RemoveTagsCtx` mutate a live stream's tag set at runtime — `streampool.go:379-429`. **This is the existing dynamic (un)subscribe hook.**
- **Memory-effectiveness levers (R2):** each stream has a **bounded** `mb.MB[drpc.Message]` queue (default size **100**), and `stream.write` uses `TryAdd` — **non-blocking, drops on overflow** — `net/streampool/stream.go:32-44`. Outbound dialing runs through a bounded `ExecPool` worker pool (`sendpool.go`). There is **no per-message persistence and no unbounded buffering** anywhere on this path.

### 4.3 Message envelope & multiplexing

- Generic space envelope `ObjectSyncMessage{spaceId, requestId, replyId, payload []byte, objectId, objectType}` — `commonspace/spacesyncproto/spacesync.pb.go:591-601`, proto at `spacesync.proto:100-108`.
- `objectId` **multiplexes many logical channels over one physical stream**; `objectType` enum is `{Tree=0, Acl=1, KeyValue=2}` — `spacesync.proto:349-353`.
- Wire wrapper `HeadUpdate` implements `drpc.Message` + a `peerMessage` tag interface (`SetPeerId`, `Copy`) so one message can be copied and stamped per destination during fan-out — `commonspace/sync/objectsync/objectmessages/headupdate.go:58-140`. Underlying `ObjectSyncMessage` is pooled via `sync.Pool` (`headupdate.go:13-38`).
- Encoding is protobuf (vtproto), optionally snappy-compressed, negotiated in handshake — `net/rpc/encoding/`.

### 4.4 The existing space-level pub/sub (**most important prior art**)

any-sync **already implements a coarse pub/sub where the "topic" is an entire `spaceId`**, over a single long-lived bidirectional stream:

- **The stream:** `rpc ObjectSyncStream(stream ObjectSyncMessage) returns (stream ObjectSyncMessage)` — bidi, long-lived, one per (peer, connection) — `spacesync.proto:40`, server iface `spacesync_drpc.pb.go:245`.
- **The subscribe control frame:** `SpaceSubscription{ SpaceIds []string; Action }` with `SpaceSubscriptionAction { Subscribe=0, Unsubscribe=1 }` — `spacesync.proto:245-256`. It is carried **inside `ObjectSyncMessage.payload` with an empty `spaceId`**.
- **The wiring** (reference impl `commonspace/spaceutils_test.go:468-540`): on `OpenStream`, the client opens `ObjectSyncStream` and immediately `Send`s a `SpaceSubscription{Subscribe, [spaceId]}`. On the receive side, `HandleMessage` sees the empty-`spaceId` frame and calls `streamPool.AddTagsCtx(ctx, spaceIds...)` — tagging the stream so subsequent `Broadcast(msg, spaceId)` reaches it. Non-control frames route to `space.HandleMessage`.
- **Server side** registers the inbound stream into its own pool: `streamPool.ReadStream(stream, 100)` — `commonspace/spacerpc_test.go:170-172` — then pushes head-updates back down the same stream via `Broadcast(msg, spaceId)`.
- **Subscribe is also kicked during head-sync**: `diffsyncer.subscribe` builds `SpaceSubscription{Subscribe}` and sends it after a successful `SpacePush` — `commonspace/headsync/diffsyncer.go:275-293`.

**Reading:** a stateless *topic* pub/sub is, structurally, a **refinement of this existing mechanism** to finer, ephemeral topics *within* a space — the same subscribe/unsubscribe control-frame pattern and the same tag-index fan-out. The delta is (i) a topic namespace below spaceId, (ii) an ephemeral message type that is **not** routed into the sync/DAG engine, and (iii) publish/subscribe ACL gating.

> **⚠️ User steer (recorded concern) — do NOT reuse `ObjectSyncStream` itself.** `ObjectSyncStream` is an *upper-level* channel: every frame on it is routed into the sync engine (`space.HandleMessage` → `SyncService.HandleMessage` → per-`objectId` `multiqueue` → `objectSync.HandleHeadUpdate`, §4.5). Multiplexing ephemeral pub/sub onto that same stream **couples pub/sub to sync**: they share one bounded queue (size 100, drop-on-overflow), so a pub/sub burst can starve or drop sync (head-of-line blocking) and vice-versa; they share the sync dispatch/backpressure path; and it muddles fire-and-forget ephemera with DAG anti-entropy correctness. **Preferred direction: a *separate* DRPC stream on a *separate* sub-connection**, with its own tags, its own read/write loops, and its own bounded queues — fully isolated from sync flow control. This is cheap on the substrate: peers already multiplex many DRPC sub-streams over a single `MultiConn` (QUIC/yamux), and maintain a pool of reusable sub-connections — a "separate sub-connection" is another *multiplexed* stream, **not** a new transport/TLS handshake (`net/transport/transport.go:40-64` `MultiConn.Open`; `net/peer/peer.go:109-110,270-297` sub-conn pool; QUIC `MaxIncomingStreams=128` `net/transport/quic/quic.go:52`). What is reusable is the *pattern* (subscribe control-frame + `StreamPool` tag-index `Broadcast`) and the `StreamPool`/`StreamHandler` plumbing — instantiated as its own stream, not layered onto the sync stream. See Section 7, tension #2.

### 4.5 Sync service dispatch

- `SyncService.BroadcastMessage` → `peerManager.BroadcastMessage` — `commonspace/sync/sync.go:97-99`.
- `SyncService.HandleMessage` enqueues onto a **per-`objectId`** `multiqueue.MultiQueue` (size 100; **overflow silently dropped** via `mb.ErrOverflowed`) → `objectSync.HandleHeadUpdate` → object resolved by `objectId` — `sync.go:101-129`, `commonspace/sync/objectsync/synchandler.go:58-79`.
- Note the existing bounded-queue + drop-on-overflow discipline is already the R2 pattern; a pub/sub path would want the same.

### 4.6 Node topology & who relays (R1 + topology decision)

- Node roles: `tree`(=sync node), `consensus`, `file`, `coordinator`, `namingNode`, `paymentProcessingNode` — `nodeconf/config.go:20-30`. A "client" is any account not in the node list.
- **Responsible nodes via consistent hash:** `NodeIds(spaceId)` returns the tree nodes on the chash ring for that space; `ReplicationFactor = 3` ⇒ **3 responsible sync nodes per space** — `nodeconf/nodeconf.go:61-79,133-176`, `nodeconf/service.go:22-25`. Only `tree` nodes are ring members.
- **Space peer set = responsible sync nodes + directly-connected clients.** Sync nodes are the *always-reachable* members; client↔client is possible (one-to-one spaces, local discovery) but not guaranteed — `commonspace/peermanager/peermanager.go:17-34`, reference `commonspace/sync/synctest/testpeermanager.go:36-66`.
- **Implication:** the natural fan-out hub for a space is its responsible sync node(s). But those nodes are **semi-trusted relays that cannot read space content** — which is why R3/encryption (Section 6) is load-bearing, and why a broker-style design here is not a trusted broker.

### 4.7 App framework (how a new component wires in)

- Component registry with `Init(a)`/`Run(ctx)`/`Close(ctx)` + `Name()`; `MustComponent[T]` lookup; **per-space child app** (`ChildApp`) gives each space an isolated component graph — `app/app.go:34-52,133-207,226-280`; space graph built in `commonspace/spaceservice.go:188-260`.
- Adding a new DRPC service is a known, mechanical path (proto + `Makefile` generate line + `DRPCRegisterXxx` on server + client component + app registration) — `Makefile:23-32,47-60`; reference client component `coordinator/subscribeclient/client.go`.

### 4.8 ACL / access control (R3)

- **Permission ladder:** `None=0, Owner=1, Admin=2, Writer=3, Reader=4, Guest=5` — `commonspace/object/acl/aclrecordproto/aclrecord.pb.go:71-79`. Helpers: `CanWrite()` = Admin|Writer|Owner; **there is no `CanRead()`** — "can read" is expressed as `!NoPermissions()` (any non-`None`) — `commonspace/object/acl/list/models.go:79-152`.
- **Authorization is cryptographic and content-based, NOT a per-message live check:**
  - **Writes** are enforced when a signed record/change is *validated on apply*: each change carries author `Identity` + signature, checked against `AclState.PermissionsAtRecord(aclHeadId, identity).CanWrite()` — `commonspace/object/tree/objecttree/objecttreevalidator.go:182-189`; KV analog `keyvaluestorage/storage.go:117`.
  - **Reads** are enforced by **encryption**: content is encrypted with a per-space `ReadKey` handed (encrypted per member pubkey) only to members inside ACL records — `commonspace/object/acl/list/aclstate.go:50-59,152-195`; rotation on membership change `aclrecordbuilder.go:761-844`.
- **Identity vs peer:** device/peer key (libp2p, authenticates `peerId` at TLS) is distinct from the **account/identity key** (the ACL `Identity`). A node's inbound handshake uses `peerSignVerifier` to prove control of the account key, placing `identity` in the connection ctx — `net/secureservice/secureservice.go:107-174`, `credential.go:67-124`.
- **KEY FINDING for R3:** the shared sync layer performs **no per-message reader-authorization** and **no ACL check at message-handle time** — a grep across `commonspace/sync`, `commonspace/spacesyncproto`, `commonspace/headsync` finds no `Permissions/CanWrite/NoPermissions` usage. Membership is gated **at stream-open by the node** (that logic lives in `any-sync-node`), and content confidentiality relies on read-key **encryption** (non-members receive ciphertext they cannot decrypt). The coordinator-side `acl.AclService.Permissions(ctx, identity, spaceId)` exists for explicit checks — `acl/acl.go:172-180`.

---

## 5. Prior art inside any-sync (what already exists to lean on or contrast)

| Prior-art mechanism | Where | Relation to stateless pub/sub |
|---|---|---|
| **Space subscription over `ObjectSyncStream`** (`SpaceSubscription{Subscribe/Unsubscribe}` + `AddTagsCtx`) | `spacesync.proto:40,245-256`; `spaceutils_test.go:468-540` | **Direct precedent** — coarse pub/sub, topic == spaceId. A topic pub/sub generalizes this. |
| **`StreamPool.Broadcast(msg, tags...)`** tag-indexed fan-out | `net/streampool/streampool.go:360-377` | The reusable fan-out engine; topics could be additional tags. |
| **Coordinator `NotifySubscribe(req) → stream NotifySubscribeEvent`** | `coordinator.proto:57`, `coordinator/subscribeclient/{client,stream}.go` | Server-push subscription-stream pattern, but **coordinator-scoped** and fixed to enum event *types* (`InboxNewMessageEvent`, `NetworkConfigChangedEvent`) — not arbitrary space topics. Good template for a dedicated pub/sub service + auto-reconnect + `mb.MB` mailbox. |
| **Coordinator Inbox** (`InboxFetch` / `InboxAddMessage`, signed sender→receiver messages) | `coordinator.proto:51-54,434-473` | **Contrast / anti-pattern for "stateless":** this is *stateful* store-and-forward (persisted, fetch-by-offset, `hasMore`). Shows what stateless pub/sub deliberately is *not*. |
| **`mb.MB[T]` bounded mailbox** (`cheggaaa/mb/v3`) | `subscribeclient/stream.go:18`; stream queues `stream.go` | The idiomatic **memory-bounded** streaming buffer (R2) — bounded size, backpressure or drop. |
| **`multiqueue.MultiQueue` per-object sharded queue, drop-on-overflow** | `commonspace/sync/sync.go:101-129` | Existing R2 discipline for per-logical-channel inbound processing. |

---

## 6. How R3 (ACL) specifically interacts with pub/sub — facts, not decisions

The spec must resolve publish-permission and subscribe-permission against these substrate facts:

- **Two distinct permissions are in play.** Publishing is a *write-like* action (`CanWrite()` ⇒ Admin/Writer/Owner). Subscribing/receiving is a *read-like* action (`!NoPermissions()` ⇒ any member incl. Reader/Guest). Presence/typing/cursors, however, are things a **Reader** plausibly should be allowed to *emit* — so "publish == CanWrite" may be too strict for the archetypal use case. **Open.**
- **No per-message ACL gate exists to reuse.** Enforcement today is either (a) node-side membership gating at stream/subscribe time, or (b) read-key encryption. A pub/sub design must choose one or both; there is no drop-in per-message reader check in the shared layer.
- **The relay node cannot be trusted with plaintext.** Consistent with the whole any-sync model, if payloads are encrypted with the space `ReadKey`, the relaying sync node routes ciphertext it cannot read — automatically enforcing read-confidentiality (non-members lack the key) at the cost of the node being unable to match on payload contents (fine) and potentially topic names (depends on whether topic strings are encrypted — **open**).
- **Key rotation on membership change is already handled** for stored content (`ReadKey` rotates, re-encrypted per remaining member — `aclrecordbuilder.go:761-844`). For *ephemeral* messages the question is whether pub/sub piggybacks the current `ReadKey` (so a removed member instantly loses the ability to decrypt new messages) or uses a separate ephemeral key. **Open.**
- **Publish authorization without a per-message check** implies either signing each ephemeral message (adds CPU + size — measure against R2) or relying on "only key-holders can produce decryptable messages" (confidentiality without authenticity — a receiver couldn't distinguish which member sent it, or prevent a Reader from spoofing). **Open trade-off.**

---

## 7. Open design tensions the spec must resolve (NOT resolved here)

Grouped by the requirement they stress. Each is a genuine fork with substrate consequences noted.

**Topology & transport (R1)**
1. **Relay vs mesh.** Fan-out through the 3 responsible sync nodes (always reachable, broker-like, but untrusted-for-content) vs direct client↔client (P2P, GossipSub-like, not always reachable) vs hybrid. Substrate favors relay-through-node for reachability; mesh fits the decentralization ethos but has no guaranteed connectivity.
2. **Dedicated pub/sub stream on its own sub-connection vs reusing `ObjectSyncStream`.** **User steer (recorded, §4.4): do not reuse `ObjectSyncStream`** — it is upper-level and tied to the sync engine, so sharing it couples pub/sub and sync (shared bounded queue, head-of-line blocking, intertwined backpressure). The preferred direction is a **separate DRPC stream over a separate multiplexed sub-connection** (cheap: another sub-stream on the existing `MultiConn`, not a new handshake), reusing only the *pattern* (subscribe control-frame + `StreamPool` tag-index `Broadcast`) and the `StreamPool`/`StreamHandler` plumbing — not the sync stream. Remaining sub-decision for the spec: does the separate stream belong to a **new dedicated `pubsub` DRPC service** (à la coordinator `NotifySubscribe`, cleanest isolation of lifecycle/versioning) or a **new stream RPC added to the existing `SpaceSync` service** (fewer moving parts, same service registration)? Both are mechanically supported (Section 4.7); both keep pub/sub off the sync stream.

**Topic model**
3. **Flat topics vs NATS-style hierarchy with wildcards** (`*`/`>` or `+`/`#`). Hierarchy+wildcards is expressive but forces the relay to run interest-matching per message (relay CPU/mem vs R2); flat topics map cleanly onto the existing tag index. If wildcards are wanted, the tag-index (`streamIdsByTag`, exact-match) is insufficient and a trie/matcher is required.
4. **Topic namespace & encryption of topic names.** Topics are scoped within a `spaceId`; are topic strings plaintext (relay can route on them but learns them) or derived/encrypted (relay routes on opaque handles)? Interacts with R3.

**Statelessness & memory (R2)**
5. **Routing statelessness (Section 1 axis b).** Relay holds a topic→subscriber interest table (bandwidth-efficient, small transient state) vs relay broadcasts all space traffic and clients filter (zero routing state, wasteful). "Memory-effective" likely means the former with strictly-bounded tables, but confirm.
6. **Backpressure policy.** The substrate default is **drop-on-overflow** (`TryAdd`, `multiqueue` drop). For at-most-once stateless semantics that is coherent — but the spec should state it explicitly (slow subscriber ⇒ dropped messages, never memory growth).
7. **Fan-out amplification.** One publish × N subscribers × up to 3 relaying nodes. Where does the copy happen (node-side fan-out preferred so the publisher uploads once)? Bounds on N, message size, publish rate.

**Delivery semantics**
8. **Plain fan-out only, or also queue-groups (1-of-N)?** Queue groups need group-membership state on the relay; likely out of scope for v1 but should be an explicit non-goal or goal.
9. **Presence lifecycle.** If presence/typing/cursors are in scope, adopt a Yjs-awareness-style **heartbeat + timeout + explicit-leave** (`null` state, ~15 s re-announce / ~30 s expiry) — otherwise "who is online" cannot be derived from fire-and-forget alone. Decide whether presence is a first-class feature or just an example payload.
10. **Reconnection.** Stateless ⇒ messages during a disconnect are lost by definition; on reconnect a subscriber re-subscribes and gets only new messages. Confirm no "catch-up" expectation (that would make it stateful).

**ACL (R3)** — the four open items in Section 6 (publish vs subscribe permission level; encryption of payload/topic; ephemeral vs space `ReadKey`; per-message signing vs encryption-only).

**Abuse resistance**
11. GossipSub-style peer scoring / rate-limiting is absent here; the substrate has a per-peer request rate-limit in `requestmanager` but nothing pub/sub-specific. Decide whether publish-rate limiting / anti-spam is in scope (a Reader flooding a topic).

---

## 8. Success criteria the spec should be measured against

- **R1:** rides existing transports + DRPC + StreamPool; no new side-channel; ideally reuses the `ObjectSyncStream`/tag machinery or cleanly mirrors the `NotifySubscribe` pattern.
- **R2:** per-connection and per-node footprint is **bounded and ephemeral** — bounded queues, drop (not buffer) on overflow, no persistence, transient routing tables sized O(active subscriptions). No path that grows memory with message volume or offline duration.
- **R3:** subscribe and publish are gated by ACL (membership + permission level), and confidentiality holds against the untrusted relay (encryption boundary preserved). A removed member loses access to new messages.
- **Semantics:** documented at-most-once, fire-and-forget, no ordering/dedup guarantees — matching the core-NATS/Redis family, explicitly *not* the stateful Inbox/DAG/KV families.

---

## 9. Sources

**any-sync (this repo, `main`)** — grounded `file:line` references inline throughout Section 4–6; key anchors: `net/streampool/streampool.go`, `commonspace/spacesyncproto/protos/spacesync.proto`, `commonspace/sync/sync.go`, `commonspace/object/acl/list/aclstate.go`, `nodeconf/nodeconf.go`, `coordinator/subscribeclient/`, `coordinator/coordinatorproto/protos/coordinator.proto`.

**External:**
- NATS — Publish-Subscribe: https://docs.nats.io/nats-concepts/core-nats/pubsub
- NATS — Subjects & wildcards: https://docs.nats.io/nats-concepts/subjects
- NATS — Queue Groups: https://docs.nats.io/nats-concepts/core-nats/queue
- libp2p GossipSub (design, mesh/fanout, IHAVE/IWANT, peer scoring): https://github.com/libp2p/specs/tree/master/pubsub/gossipsub
- Yjs awareness protocol: https://github.com/yjs/y-protocols/blob/master/PROTOCOL.md and https://docs.yjs.dev/api/about-awareness
- Redis pub/sub: https://redis.io/docs/latest/develop/interact/pubsub/
- MQTT topics/wildcards/QoS: https://mqtt.org/ (spec)
- Phoenix Channels & Presence: https://hexdocs.pm/phoenix/Phoenix.Channel.html , https://hexdocs.pm/phoenix/Phoenix.Presence.html
- Matrix ephemeral events (typing/presence EDUs): https://spec.matrix.org/ (server-server EDUs)
