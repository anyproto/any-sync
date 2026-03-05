# Protocol Versioning and Release Guide

## Overview

Each any-sync release declares a `ProtoVersion` and a list of `compatibleVersions`.
During the handshake, both sides exchange their `ProtoVersion` and check it against
their own `compatibleVersions` list. Both sides must accept each other for the
connection to succeed.

### Version alignment rule

`ProtoVersion` equals the **minor** component of the any-sync semver version:

| any-sync version | ProtoVersion |
|------------------|--------------|
| v0.12.X          | 12           |
| v0.13.X          | 13           |
| v0.14.X          | 14           |

Patch releases (`v0.X.Y`) never change `ProtoVersion`.

### Default compatibility range

Each version accepts **itself ±1** by default:

```go
// v0.13.X
ProtoVersion       = uint32(13)
compatibleVersions = []uint32{12, 13, 14}  // V-1, V, V+1
```

This can be overridden per-node via configuration:

```yaml
secureService:
  compatibleVersions: [12, 13]
```

### Client status indicators

After a successful handshake, the client compares its `ProtoVersion` with the
remote peer's version:

| Condition | Status | Action |
|-----------|--------|--------|
| Versions match | **Green** | None |
| Versions differ but compatible | **Yellow** | Prompt user to update |
| Version not in compatible list | **Red** | Cannot sync |

The yellow warning is triggered by `IsNetworkNeedsUpdate()` which returns `true`
when the infra's proto version is higher than the client's.

---

## Releasing an incompatible feature

Incompatible features require a **two-version rollout**: prepare the handler code
in version V+1, activate in V+2. This guarantees that all connected peers can
handle the new data before anyone starts producing it.

### Phase 1: PREPARE (v0.N.0)

Ship the code that **handles** the new data format (can receive/read it) but do
**not** produce it yet.

```
ProtoVersion       = N
compatibleVersions = [N-1, N, N+1]
```

**Step 1 — Release clients v0.N.0**

| | infra N-1 (compat [N-2, N-1, N]) |
|---|---|
| v0.N.0 client (N, compat [N-1, N, N+1]) | **green** — N∈[N-2,N-1,N], N-1∈[N-1,N,N+1] |
| old client (N-1) | **green** |

No warnings anywhere. Infra is still at N-1.

**Step 2 — Deploy infra v0.N.0**

| | infra N (compat [N-1, N, N+1]) |
|---|---|
| v0.N.0 client (N) | **green** |
| old client (N-1, compat [N-2, N-1, N]) | **yellow** — update available in stores |

### Phase 2: ACTIVATE (v0.N+1.0)

Enable the feature. Clients start **producing** the new data format.

**Step 3a — Infra config: drop N-1**

This is a configuration-only change on infra nodes. No code release needed.

```yaml
secureService:
  compatibleVersions: [N, N+1]
```

| | infra N (config compat [N, N+1]) |
|---|---|
| v0.N.0 client (N) | **green** |
| old client (N-1) | **red** — cannot sync |

Clients on N-1 already had the yellow warning since step 2 and had time to update.

**Step 3b — Release clients v0.N+1.0**

Feature is **activated**.

```
ProtoVersion       = N+1
compatibleVersions = [N, N+1, N+2]
```

| | infra N (config compat [N, N+1]) |
|---|---|
| v0.N+1.0 client (N+1) | **green** — N+1∈[N,N+1], N∈[N,N+1,N+2] |
| v0.N.0 client (N) | **green** |

v0.N.0 clients handle the new data correctly (they got the handler code in phase 1).

**Step 3c — Deploy infra v0.N+1.0**

After clients v0.N+1.0 are released. Remove the config override.

| | infra N+1 (compat [N, N+1, N+2]) |
|---|---|
| v0.N+1.0 client (N+1) | **green** |
| v0.N.0 client (N, compat [N-1, N, N+1]) | **yellow** — update available |

### Summary

| Step | Action | Deployed version | N-1 clients | N clients | N+1 clients |
|------|--------|------------------|-------------|-----------|-------------|
| 1 | Release clients v0.N.0 | clients=N, infra=N-1 | green | green | — |
| 2 | Deploy infra v0.N.0 | clients=N, infra=N | yellow | green | — |
| 3a | Infra config: drop N-1 | infra config=[N, N+1] | **red** | green | — |
| 3b | Release clients v0.N+1.0 | clients=N+1, infra=N | red | green | green |
| 3c | Deploy infra v0.N+1.0 | clients=N+1, infra=N+1 | red | yellow | green |

### Key rules

- **Clients first, infra last** — never show a yellow warning before the update
  is available.
- **Drop old version (3a) before activating new feature (3b)** — clients that
  cannot handle the new data must be disconnected before anyone starts producing it.
- **One in-flight upgrade at a time** — do not start the next proto bump until
  all clients have shipped the current one.

---

## Example: delivering two incompatible features

This example shows how to deliver **Feature A** (in v0.14) and **Feature B**
(in v0.15) starting from a stable v0.13 baseline.

### Starting state

Everyone is on v0.13.0:
```
ProtoVersion       = 13
compatibleVersions = [12, 13, 14]
```

### Feature A: prepare in v0.14, activate in v0.15

**v0.14.0 — PREPARE Feature A**

Ship handler code for Feature A (can receive, does not produce).

```
ProtoVersion       = 14
compatibleVersions = [13, 14, 15]
```

| Step | Action | v12 | v13 | v14 |
|------|--------|-----|-----|-----|
| 1 | Release clients v0.14.0 | red | green | green |
| 2 | Deploy infra v0.14.0 | red | yellow | green |

At this point v12 clients are already red (dropped at v0.13→v0.14 transition).
v13 clients see yellow, update is available.

**v0.15.0 — ACTIVATE Feature A + PREPARE Feature B**

Both features can share a version bump: activate A and prepare handler code for B.

```
ProtoVersion       = 15
compatibleVersions = [14, 15, 16]
```

| Step | Action | v13 | v14 | v15 |
|------|--------|-----|-----|-----|
| 3a | Infra config: drop v13 | **red** | green | — |
| 3b | Release clients v0.15.0 | red | green | green |
| 3c | Deploy infra v0.15.0 | red | yellow | green |

Feature A is now active on v15 clients. v14 clients handle the data correctly.
Feature B handler code is shipped but not activated yet.

### Feature B: activate in v0.16

**v0.16.0 — ACTIVATE Feature B**

```
ProtoVersion       = 16
compatibleVersions = [15, 16, 17]
```

| Step | Action | v14 | v15 | v16 |
|------|--------|-----|-----|-----|
| 4a | Infra config: drop v14 | **red** | green | — |
| 4b | Release clients v0.16.0 | red | green | green |
| 4c | Deploy infra v0.16.0 | red | yellow | green |

Feature B is now active. v15 clients handle it correctly.

### Timeline visualization

```
v0.13  ──────────────────────────────────────────────────
         stable baseline

v0.14  ──────────────────────────────────────────────────
         Feature A handler code shipped (PREPARE)
         Feature A NOT active yet

v0.15  ──────────────────────────────────────────────────
         Feature A ACTIVATED (clients produce new data)
         Feature B handler code shipped (PREPARE)
         Feature B NOT active yet

v0.16  ──────────────────────────────────────────────────
         Feature B ACTIVATED
         (can start preparing Feature C here)
```

### Key observations

- **Each version carries one PREPARE and one ACTIVATE.** Starting from v0.15,
  every minor release can simultaneously activate the previous feature and
  prepare the next one. This gives a steady cadence of one incompatible feature
  per minor version.

- **Features that don't conflict can share a version.** If Feature A and
  Feature B are independent, they can both be prepared in v0.14 and both
  activated in v0.15 — no need to use separate versions.

- **Compatible features don't need a version bump.** Backward-compatible changes
  (new optional fields, additive APIs) can ship in any patch release without
  touching `ProtoVersion`.

---

## Special case: jump from v0.11.X (ProtoVersion=8) to aligned versioning

Current state: `ProtoVersion=8`, `compatibleVersions=[8, 9]`.
Target: align `ProtoVersion` with the minor version number starting at v0.12.0.

The gap from 8 to 12 requires a **bridge release** so that existing clients learn
to accept proto version 12 before infra starts announcing it.

### Step 0 — Bridge release: v0.11.Y

Code changes:
- Add `CompatibleVersions` field to `secureservice.Config` (allows infra to
  override compatible versions via configuration)
- Change default: `compatibleVersions = [8, 9, 12]`
- Include feature handler code (PREPARE)

Deploy infra, release clients. Wait for adoption.

```
ProtoVersion       = 8
compatibleVersions = [8, 9, 12]
```

| | infra 8 (compat [8, 9, 12]) |
|---|---|
| v0.11.Y client (8, compat [8, 9, 12]) | **green** |
| old client (8, compat [8, 9]) | **green** |

Everyone stays green. No version bump, no warnings.

### Step 1 — Release clients v0.12.0

```
ProtoVersion       = 12
compatibleVersions = [8, 12, 13]
```

| | infra 8 (compat [8, 9, 12]) |
|---|---|
| v0.12.0 client (12, compat [8, 12, 13]) | **green** — 12∈[8,9,12], 8∈[8,12,13] |
| v0.11.Y client (8, compat [8, 9, 12]) | **green** |

No warnings. Infra is still at proto 8, clients are ahead.

### Step 2 — Deploy infra v0.12.0

After clients v0.12.0 are released.

```
ProtoVersion       = 12
compatibleVersions = [8, 12, 13]
```

| | infra 12 (compat [8, 12, 13]) |
|---|---|
| v0.12.0 client (12) | **green** |
| v0.11.Y client (8, compat [8, 9, 12]) | **yellow** — 12∈[8,9,12], update available |
| pre-bridge client (8, compat [8, 9]) | **red** — 12∉[8,9] |

Clients that never updated to v0.11.Y go directly to red.

### Step 3a — Infra config: drop v8

```yaml
secureService:
  compatibleVersions: [12, 13]
```

| | infra 12 (config compat [12, 13]) |
|---|---|
| v0.12.0 client (12) | **green** |
| v0.11.Y client (8) | **red** — 8∉[12,13] |

### Step 3b — Release clients v0.13.0

Feature **activated**.

```
ProtoVersion       = 13
compatibleVersions = [12, 13, 14]
```

| | infra 12 (config compat [12, 13]) |
|---|---|
| v0.13.0 client (13) | **green** — 13∈[12,13], 12∈[12,13,14] |
| v0.12.0 client (12) | **green** |

### Step 3c — Deploy infra v0.13.0

After clients v0.13.0 are released. Remove config override.

```
ProtoVersion       = 13
compatibleVersions = [12, 13, 14]
```

| | infra 13 (compat [12, 13, 14]) |
|---|---|
| v0.13.0 client (13) | **green** |
| v0.12.0 client (12, compat [8, 12, 13]) | **yellow** — 13∈[8,12,13], update available |

From this point forward, the standard ±1 flow applies for all future versions.
