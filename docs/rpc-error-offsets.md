# RPC Error Code Offsets

## Overview

any-sync maps typed RPC errors onto numeric [dRPC](https://storj.io/drpc) error codes
through a single **process-global** registry in
[`net/rpc/rpcerr`](../net/rpc/rpcerr/registry.go).

```go
// net/rpc/rpcerr/registry.go
var errsMap = make(map[uint64]error) // global, one per process

func RegisterErr(err error, code uint64) error {
    if e, ok := errsMap[code]; ok {
        panic(fmt.Errorf("attempt to register error with existing code: %d ...", code))
    }
    // ...
}

type ErrGroup int64
func (g ErrGroup) Register(err error, code uint64) error {
    return RegisterErr(err, uint64(g)+code) // registers at offset+code
}
```

Each proto/service declares an `ErrorOffset` constant and registers its errors
through `rpcerr.ErrGroup(ErrorOffset)`; a concrete error's wire code is
`ErrorOffset + localCode`.

### Why this needs coordinating across repos

`errsMap` is **global to the running binary**, and registration happens in
`var`/`init` blocks at import time. A node binary routinely links **any-sync
plus one or more downstream repos** (e.g. `any-sync-node` also registers
`nodesync` errors). If any two groups ever register the same `offset + code`,
the program **panics at startup** — before any request is served.

So offsets are a shared namespace across the whole anyproto ecosystem, not just
within one repo. **This file is the source of truth for who owns which offset.**

## Allocation rules

1. **Offsets are multiples of 100.** Each group owns the band
   `[offset, offset+99]`.
2. **Local codes must stay in `0..99`** so a group never bleeds into the next
   band. (The `ErrorOffset` enum value itself is only the base — it is never
   registered as a code.)
3. **Local code `0` is the group's `Unexpected`/fallback** (registers at
   `offset+0`), mirroring `filesync.ErrCodes.Unexpected = 0`.
4. **any-sync core uses `< 1000`.** Downstream repos that import any-sync must
   use **`>= 1000`** to stay clear of the core range.
5. When you add a group, **pick the next free offset and add a row below.**

### Globally reserved low codes

The base registry claims two codes directly (outside any group), so **no group
may use offset `0`**:

| Code | Name | Source |
|------|------|--------|
| 1 | `Unexpected` | `net/rpc/rpcerr/registry.go` |
| 2 | `Closed` | `net/rpc/rpcerr/registry.go` |

## Reserved offsets

### any-sync (core, `< 1000`)

| Offset | Band | Proto / service | Package |
|--------|------|-----------------|---------|
| 100 | 100–199 | `spacesync` | `commonspace/spacesyncproto` |
| 200 | 200–299 | `filesync` (File, v1) | `commonfile/fileproto` |
| 300 | 300–399 | `coordinator` | `coordinator/coordinatorproto` |
| 400 | 400–499 | `treechange` | `commonspace/object/tree/treechangeproto` |
| 500 | 500–599 | `consensus` | `consensus/consensusproto` |
| 600 | 600–699 | `payment` | `paymentservice/paymentserviceproto` |
| 700 | 700–799 | `limiter` | `net/rpc/limiter/limiterproto` |
| 800 | 800–899 | `filesyncv2` (FileV2, v2 broker) | `commonfile/fileproto/fileprotov2` |
| 900 | 900–999 | _free_ | — |

### Downstream repos (`>= 1000`)

These live in other repositories but share this global registry whenever their
binary also links any-sync.

| Offset | Band | Proto / service | Repo · package |
|--------|------|-----------------|----------------|
| 1000 | 1000–1099 | `nodesync` | `any-sync-node` · `nodesync/nodesyncproto` |
| 1100 | 1100–1199 | _free_ | — |
| 1200 | 1200–1299 | `push` | `anytype-push-server` · `pushclient/pushapi` |
| 1300 | 1300–1399 | `bobrik` | `bobrik-clusterctl` · `bobrikclient/bobrikapi` |
| 1400+ | — | _free_ | — |

## Adding a new group

1. Choose the next free offset from the tables above (core: next free `< 1000`;
   downstream: next free `>= 1000`).
2. In the `.proto`, add `ErrorOffset = <offset>;` to the service's `ErrCodes`
   enum, with local codes `0..N` (`0` = `Unexpected`).
3. Register in a Go errors file via `rpcerr.ErrGroup(<Enum>_ErrorOffset)`
   (see `commonfile/fileproto/fileprotov2/fileprotov2err/fileprotov2err.go` for
   the current template).
4. **Add a row to the correct table in this file.**
5. Build and run any test — a colliding offset panics at init, so a green test
   run proves the new band is clear.
