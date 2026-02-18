# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`any-sync` is a Go library implementing an open-source P2P synchronization protocol for encrypted communication channels (spaces). Data is stored as encrypted DAGs using CRDTs for conflict-free resolution. This is a library, not a standalone application — it is consumed by `any-sync-node`, `any-sync-filenode`, `any-sync-consensusnode`, and `any-sync-coordinator`.

## Common Commands

### Build & Test
```bash
# Run all tests (requires GOEXPERIMENT=synctest until Go 1.25)
make test
# Or directly:
GOEXPERIMENT=synctest go test ./... --cover

# Run a single test
GOEXPERIMENT=synctest go test ./commonspace/... -run TestSpecificName

# Run tests in a specific package
GOEXPERIMENT=synctest go test ./app/ocache/...
```

### Code Generation
```bash
# Build protoc plugins and mockgen into ./deps/
make deps

# Regenerate protobuf Go code from .proto files
make proto

# Regenerate mocks (runs go generate ./...)
make mocks
```

### Setup
```bash
# Configure git hooks (gitleaks pre-push)
make all
```

## Architecture

### Component System (`app/`)

The `app.App` struct is a dependency injection container managing component lifecycle:

1. `app.Register(component)` — registers a component
2. `component.Init(app)` — called in registration order; components look up dependencies via `app.MustComponent(name)`
3. `component.Run(ctx)` — called after all Init; for `ComponentRunnable` only
4. `component.Close(ctx)` — called in reverse order on shutdown

Every service implements the `Component` interface (with `Init` + `Name`), and optionally `ComponentRunnable` (adds `Run` + `Close`). Components expose a `CName` constant for lookup.

### Core Domain Packages

- **`commonspace/`** — Central package. `Space` interface represents a sync channel containing ACL, trees, key-value storage, and sync machinery. Subpackages:
  - `object/acl/` — cryptographic access control lists with permission records
  - `object/tree/` — CRDT-based DAG trees (objecttree, synctree)
  - `object/keyvalue/` — key-value storage within spaces
  - `headsync/` — head-based synchronization protocol
  - `spacesyncproto/` — main protobuf-defined RPC service (SpacePush, SpacePull, ObjectSyncStream, etc.)
  - `sync/` — sync service orchestrating object and head sync
- **`commonfile/`** — file synchronization over the protocol
- **`net/`** — networking layer: peer management, secure handshakes, stream pooling, RPC server/client
- **`consensus/`** — consensus node client for ACL validation
- **`coordinator/`** — coordinator node client for network topology
- **`acl/`** — higher-level ACL service wrapping the low-level ACL list
- **`util/crypto/`** — cryptographic primitives (Ed25519 keys, signing, encryption)
- **`nodeconf/`** — node configuration and network topology

### RPC & Protobuf

Uses dRPC (storj.io/drpc) instead of gRPC for RPC. Proto files live in `<package>/protos/*.proto` directories. Code generation produces three outputs:
- Standard Go protobuf types (`protoc-gen-go`)
- dRPC service stubs (`protoc-gen-go-drpc`)
- VTProto optimized marshal/unmarshal (`protoc-gen-go-vtproto` with `marshal+unmarshal+size`)

### Mocking

Uses `go.uber.org/mock/mockgen` via `//go:generate` directives. Mock files are colocated with source (typically `mock_<package>/` directories or inline). Regenerate with `make mocks`.

### Testing

- Uses `testify` for assertions (`require` and `assert`)
- `testutil/accounttest/` provides test account/identity generation helpers
- `net/` contains end-to-end test infrastructure in `endtoendtest/` and `streampool/testservice/`
- The `GOEXPERIMENT=synctest` flag is required for tests that use Go's experimental sync testing

### Logging

Uses `go.uber.org/zap` with the pattern:
```go
var log = logger.NewNamed("packagename")
```

## Key Dependencies

- `storj.io/drpc` — RPC framework (used instead of gRPC)
- `github.com/anyproto/any-store` — local storage engine
- `github.com/planetscale/vtprotobuf` — fast protobuf serialization
- `github.com/libp2p/go-libp2p` — P2P networking
- `github.com/quic-go/quic-go` — QUIC transport

## Module

```
module github.com/anyproto/any-sync
GOPRIVATE=github.com/anyproto
```
