# Protocol Versioning and Release Guide

## Overview

Each any-sync release declares a `ProtoVersion` and a list of `compatibleVersions`.
During the handshake both sides exchange their `ProtoVersion` and check it against
their own `compatibleVersions` list. Both sides must accept each other for the
connection to succeed.

### Version alignment rule

`ProtoVersion` equals the **minor** component of the any-sync semver version.
Patch releases never change `ProtoVersion`.

| any-sync version | ProtoVersion |
|------------------|--------------|
| v0.12.X          | 12           |
| v0.13.X          | 13           |
| v0.14.X          | 14           |

### Default compatibility range

Each version accepts **itself ±1** by default:

```go
// v0.13.X
ProtoVersion       = uint32(13)
compatibleVersions = []uint32{12, 13, 14}  // V-1, V, V+1
```

Override per-node via configuration:

```yaml
secureService:
  compatibleVersions: [12, 13]
```

### Client status

After a successful handshake the client compares its `ProtoVersion` with the remote peer's version.
Yellow is triggered only when infra version is **higher** than client's (`IsNetworkNeedsUpdate()`).
If client is higher but still compatible — green.

| Condition | Status | Action |
|-----------|--------|--------|
| Versions match | **green** | None |
| Infra version higher, but compatible | **yellow** | Prompt user to update |
| Version not in compatible list | **red** | Cannot sync |

---

## Releasing an incompatible feature

Two-version rollout: **PREPARE** handler code in version N, **ACTIVATE** in version N+1.
All connected peers can handle the new data before anyone starts producing it.

### Rollout steps

| Step | Action | Proto / compat | N-1 | N | N+1 |
|------|--------|----------------|-----|---|-----|
| 1 | Release clients v0.N.0 (PREPARE) | N / [N-1, N, N+1] | green | green | — |
| 2 | Deploy infra v0.N.0 | N / [N-1, N, N+1] | yellow | green | — |
| 3a | Infra config: drop N-1 | N / [N, N+1] | **red** | green | — |
| 3b | Release clients v0.N+1.0 (ACTIVATE) | N+1 / [N, N+1, N+2] | red | green | green |
| 3c | Deploy infra v0.N+1.0, remove config override | N+1 / [N, N+1, N+2] | red | yellow | green |

### Key rules

- **Clients first, infra last** — never show yellow before the update is available in stores. Wait until all platforms (iOS, Android, desktop) are live.
- **Drop old version (3a) before activating (3b)** — clients that cannot handle new data must be disconnected before anyone produces it.
- **Gate on metrics** — proceed to 3a/3c when old-version client share drops to an acceptable level.
- **One in-flight upgrade at a time** — do not start the next proto bump until all clients have shipped the current one.

---

## Example: two incompatible features

Starting from stable v0.13 baseline. Feature A prepares in v0.14, activates in v0.15.
Feature B prepares in v0.15, activates in v0.16.

| Step | Action | v12 | v13 | v14 | v15 | v16 |
|------|--------|-----|-----|-----|-----|-----|
| 1 | Release clients v0.14.0 (PREPARE A) | red | green | green | — | — |
| 2 | Deploy infra v0.14.0 | red | yellow | green | — | — |
| 3a | Infra config: drop v13 | red | **red** | green | — | — |
| 3b | Release clients v0.15.0 (ACTIVATE A + PREPARE B) | red | red | green | green | — |
| 3c | Deploy infra v0.15.0 | red | red | yellow | green | — |
| 4a | Infra config: drop v14 | red | red | **red** | green | — |
| 4b | Release clients v0.16.0 (ACTIVATE B) | red | red | red | green | green |
| 4c | Deploy infra v0.16.0 | red | red | red | yellow | green |

**Notes:**
- Each minor release can simultaneously ACTIVATE the previous feature and PREPARE the next one.
- Independent features can share a version bump (both PREPARE in v0.14, both ACTIVATE in v0.15).
- Backward-compatible changes (new optional fields, additive APIs) ship in any patch release without touching `ProtoVersion`.

---

## Special case: jump from v0.11.X (ProtoVersion=8) to aligned versioning

Current state: `ProtoVersion=8`, `compatibleVersions=[8, 9]`.
Target: align `ProtoVersion` with minor version starting at v0.12.0.

The gap 8→12 requires a **bridge release**. Proto versions 9–11 are never used
as `ProtoVersion` for a release — 9 is used only in the bridge to distinguish
bridge clients from pre-bridge clients.

### Rollout

| Step | Action | Proto / compat | pre-bridge (v8) | bridge v0.11.Y (v9) | v0.12.0 (v12) |
|------|--------|----------------|-----------------|----------------------|---------------|
| 0 | Release bridge v0.11.Y (PREPARE) | 9 / [8, 9, 12] | green | green | — |
| 1 | Release clients v0.12.0 (ACTIVATE) | 12 / [9, 12, 13] | green | green | green |
| 2 | Deploy infra v0.12.0 | 12 / [9, 12, 13] | **red** | yellow | green |
| 3a | Infra config: drop v9 | 12 / [12, 13] | red | **red** | green |

### Step details

**Step 0 — Bridge release v0.11.Y (PREPARE)**

Code changes:
- Add `CompatibleVersions` field to `secureservice.Config` (infra config override)
- Set `ProtoVersion = 9`, `compatibleVersions = [8, 9, 12]`
- Ship handler code for new features (can receive, does not produce)

Bridge clients (proto 9) are distinguishable from pre-bridge (proto 8) on infra side,
enabling metrics tracking and selective config gating.

**Step 1 — Release clients v0.12.0 (ACTIVATE)**

Features go live. `compatibleVersions = [9, 12, 13]` — accepts bridge clients (v9)
who can handle the new data (got PREPARE in step 0). Infra still at v8, accepts v9 and v12.

**Step 2 — Deploy infra v0.12.0**

`compatibleVersions = [9, 12, 13]`. Pre-bridge clients (proto 8) go red — 8 is not
in [9, 12, 13]. Bridge clients (proto 9) go yellow — still connected and can handle
new-format data thanks to PREPARE code, prompted to update.

**Step 3a — Infra config: drop v9**

```yaml
secureService:
  compatibleVersions: [12, 13]
```

Gate on metrics: proceed when bridge client (v9) share is acceptably low.

From here the standard ±1 flow applies for all future versions.
