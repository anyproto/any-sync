# WebTransport Transport for any-sync

## Overview
- Add a WebTransport transport to any-sync, enabling node communication over HTTP/3 WebTransport
- Uses real domain SSL certificates for TLS; peer identity verified via application-level handshake (like WebRTC)
- Closely mirrors the existing QUIC transport pattern (`net/transport/quic/`)
- Includes WASM/browser dial-only support (like `webrtc_js.go`)
- Full PeerService integration with `webtransport://` address scheme
- CORS support on the HTTP/3 endpoint
- HTTP path: `/webtransport`

## Context (from discovery)
- Files/components involved:
  - `net/transport/transport.go` — transport interfaces, add `WebTransport` constant
  - `net/transport/quic/` — primary reference implementation (closest pattern)
  - `net/transport/webrtc/` — reference for browser/WASM support and app-level handshake pattern
  - `net/peerservice/peerservice.go` — transport registration and dial routing
  - `go.mod` / `go.sum` — add `webtransport-go` dependency
- Library: `github.com/quic-go/webtransport-go` (uses same `quic-go v0.59.0`)
- Key API: `webtransport.Server.Upgrade()` returns `*Session`; `webtransport.Dialer.Dial()` returns `*Session`; `Session.OpenStreamSync()`/`AcceptStream()` for multiplexed streams

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Make small, focused changes
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
- **CRITICAL: all tests must pass before starting next task** — no exceptions
- **CRITICAL: update this plan file when scope changes during implementation**
- Run tests after each change
- Maintain backward compatibility

## Testing Strategy
- **Unit tests**: required for every task
- Test infrastructure follows `quic_test.go` pattern (fixture with app, transport, test accepter)
- Tests use self-signed certs generated at test time (production uses real certs)

## Progress Tracking
- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix
- Update plan if implementation deviates from original scope

## Implementation Steps

### Task 1: Add dependency and transport constant
- [x] Add `github.com/quic-go/webtransport-go` to `go.mod` and run `go mod tidy`
- [x] Add `WebTransport = "webtransport"` constant to `net/transport/transport.go`
- [x] Verify `go build ./...` passes
- [x] Run existing tests — must pass before next task

### Task 2: Create config and interface (`net/transport/webtransport/`)
- [x] Create `net/transport/webtransport/webtransport.go`:
  - `CName = "net.transport.webtransport"`
  - `WebTransport` interface (embeds `transport.Transport` + `app.ComponentRunnable`)
  - `New()` constructor
- [x] Create `net/transport/webtransport/config.go`:
  - `configGetter` interface with `GetWebTransport() Config`
  - `Config` struct: `ListenAddrs []string`, `Path string`, `CertFile string`, `KeyFile string`, `WriteTimeoutSec int`, `CloseTimeoutSec int`, `DialTimeoutSec int`, `MaxStreams int64`
- [x] Verify `go build ./net/transport/webtransport/...` passes
- [x] Run project tests — must pass before next task

### Task 3: Implement conn.go — MultiConn and stream wrappers
- [x] Create `net/transport/webtransport/conn.go`:
  - `wtAddr` implementing `net.Addr` (Network="webtransport", String=addr)
  - `wtNetConn` struct wrapping `*webtransport.Stream` as `net.Conn`:
    - `Read`, `Write` (with optional write deadline), `Close` (CancelRead + Close, like quicNetConn)
    - `LocalAddr`, `RemoteAddr`, `SetDeadline`, `SetReadDeadline`, `SetWriteDeadline`
  - `newConn()` function creating `wtMultiConn`
  - `wtMultiConn` struct implementing `transport.MultiConn`:
    - Fields: `cctx context.Context`, `session *webtransport.Session`, `writeTimeout`, `closeTimeout`, `remoteAddr`
    - `Context()` → return enriched cctx (with `peer.CtxWithPeerAddr`)
    - `Accept()` → `session.AcceptStream(ctx)` → wrap as `wtNetConn`
    - `Open(ctx)` → `session.OpenStreamSync(ctx)` → wrap as `wtNetConn`
    - `Addr()` → `"webtransport://" + remoteAddr`
    - `IsClosed()` → check session context done
    - `CloseChan()` → `session.Context().Done()`
    - `Close()` → `session.CloseWithError(0, "")`
- [x] Write tests for `wtNetConn` (read/write/close)
- [x] Write tests for `wtMultiConn` (open/accept/close/addr)
- [x] Run project tests — must pass before next task

### Task 4: Implement native server + client (`webtransport_native.go`)
- [ ] Create `net/transport/webtransport/webtransport_native.go` (build tag `//go:build !js`):
  - `wtTransport` struct:
    - Fields: `secure secureservice.SecureService`, `localPeerId string`, `accepter transport.Accepter`, `conf Config`, `server *webtransport.Server`, `udpConns []net.PacketConn`, `listCtx/listCtxCancel`, `mu sync.Mutex`
  - `Init(app)`:
    - Get `secureservice.SecureService` via MustComponent
    - Get `accountservice.Service` for localPeerId
    - Get config from optional configGetter
    - Set defaults: Path="/webtransport", CloseTimeoutSec=5, DialTimeoutSec=30, MaxStreams=128
  - `Name()` → CName
  - `SetAccepter(accepter)`
  - `Run(ctx)`:
    - Create `tls.Config` with `GetCertificate` callback that re-reads `CertFile`/`KeyFile` from disk on each new connection (hot-reload for certbot rotation, no restart needed)
    - Create `http3.Server` with TLS config and QUIC config (EnableDatagrams, MaxIncomingStreams)
    - Call `webtransport.ConfigureHTTP3Server`
    - Create `webtransport.Server{H3: &h3Server, CheckOrigin: allow all}`
    - Register HTTP handler at `conf.Path` that calls `handleUpgrade`
    - For each listen addr: `net.ListenUDP` → `server.Serve(udpConn)` in goroutine
  - `handleUpgrade(w, r)`:
    - Add CORS headers (Access-Control-Allow-Origin: *, etc.)
    - Handle OPTIONS preflight
    - Call `server.Upgrade(w, r)` → get session
    - Launch goroutine: `accept(session, remoteAddr)`
  - `accept(session, remoteAddr)`:
    - Timeout context from DialTimeoutSec
    - `session.AcceptStream(ctx)` — handshake stream
    - Wrap as net.Conn
    - `secure.HandshakeInbound(ctx, stream, "")` — empty peerId, handshake discovers it
    - Close handshake stream
    - Create `wtMultiConn`, call `accepter.Accept(mc)`
  - `Dial(ctx, addr)`:
    - Get `expectedPeerId` from context
    - Create `webtransport.Dialer{TLSClientConfig: &tls.Config{}, QUICConfig: quicConf}`
    - `dialer.Dial(ctx, "https://"+addr+conf.Path, nil)`
    - `session.OpenStreamSync(ctx)` — handshake stream
    - Wrap as net.Conn, `secure.HandshakeOutbound(ctx, stream, expectedPeerId)`
    - Close handshake stream
    - Return `newConn(cctx, session, addr, ...)`
  - `Close(ctx)`:
    - Cancel listCtx
    - `server.Close()`
    - Close UDP connections
- [ ] Write tests for Dial + Accept flow (fixture pattern from quic_test.go, self-signed test certs)
- [ ] Write test for cert hot-reload (replace cert files, verify new connections use new cert)
- [ ] Write tests for CORS headers
- [ ] Write tests for close/shutdown behavior
- [ ] Run project tests — must pass before next task

### Task 5: Implement WASM/browser client (`webtransport_js.go`)
- [ ] Create `net/transport/webtransport/webtransport_js.go` (build tag `//go:build js`):
  - Dial-only transport (no Run/Close/listener)
  - `Init(app)` — get secureservice, localPeerId, config
  - `Dial(ctx, addr)`:
    - Use browser's WebTransport API via `syscall/js`
    - Create WebTransport connection to `https://addr/webtransport`
    - Wait for `ready` promise
    - Open bidirectional stream for handshake
    - `secure.HandshakeOutbound(ctx, stream, expectedPeerId)`
    - Return MultiConn wrapping the WebTransport session
  - `SetAccepter` — no-op (dial-only)
  - `Run` / `Close` — no-op
- [ ] Verify WASM build: `GOOS=js GOARCH=wasm go build ./net/transport/webtransport/...`
- [ ] Run project tests (native) — must pass before next task

### Task 6: PeerService integration
- [ ] Modify `net/peerservice/peerservice.go`:
  - Add import for `"github.com/anyproto/any-sync/net/transport/webtransport"`
  - Add field `webtransport transport.Transport` to `peerService` struct
  - In `Init()`: register webtransport (optional, like webrtc):
    ```go
    if comp := a.Component(webtransport.CName); comp != nil {
        p.webtransport = comp.(transport.Transport)
        p.webtransport.SetAccepter(p)
    }
    ```
  - In `preferredSchemes()`: add `transport.WebTransport` after webrtc
  - In `dialScheme()`: add case `transport.WebTransport: tr = p.webtransport`
- [ ] Write test verifying webtransport is discovered and routed in dialScheme
- [ ] Run project tests — must pass before next task

### Task 7: Verify acceptance criteria
- [ ] Verify Go-to-Go dial/accept works end-to-end (test with self-signed certs)
- [ ] Verify stream multiplexing (open multiple streams, exchange data)
- [ ] Verify handshake identity verification (correct peerId, mismatched peerId rejected)
- [ ] Verify WASM builds (`GOOS=js GOARCH=wasm go build ./net/transport/webtransport/...`)
- [ ] Verify CORS headers present on server responses
- [ ] Run full test suite: `GOEXPERIMENT=synctest go test ./... --cover`
- [ ] Run linter — all issues must be fixed
- [ ] Verify no regressions in existing QUIC/WebRTC/Yamux transports

### Task 8: [Final] Update documentation
- [ ] Update README if needed
- [ ] Add config example for WebTransport in documentation or comments

## Technical Details

### Address Format
- Peer addresses: `webtransport://host:port`
- Internal dial URL: `https://host:port/webtransport`
- Path is configurable (default: `/webtransport`)

### Connection Flow
```
Client                                    Server
  |                                         |
  |-- QUIC+TLS (real SSL cert) ------------>|
  |-- HTTP/3 CONNECT /webtransport -------->|
  |<-------- 200 OK (Upgrade) -------------|
  |                                         |
  |== WebTransport Session Established =====|
  |                                         |
  |-- OpenStream (handshake) -------------->|
  |-- HandshakeOutbound(expectedPeerId) --->|
  |<--- HandshakeInbound() ----------------|
  |-- Close handshake stream -------------->|
  |                                         |
  |== MultiConn Ready (Open/Accept) ========|
```

### Config Structure
```yaml
webtransport:
  listenAddrs: ["0.0.0.0:443"]
  path: "/webtransport"
  certFile: "/path/to/cert.pem"
  keyFile: "/path/to/key.pem"
  writeTimeoutSec: 10
  closeTimeoutSec: 5
  dialTimeoutSec: 30
  maxStreams: 128
```

### Stream Wrapping
`webtransport.Stream` → `wtNetConn` (net.Conn):
- `Close()` calls `CancelRead(0)` + `Close()` (same pattern as QUIC's quicNetConn)
- Write deadline support via `SetWriteDeadline`

### WASM/Browser Support
- Build tag: `//go:build js` for WASM, `//go:build !js` for native
- WASM is dial-only (no server/listener)
- Uses browser's `WebTransport` API via `syscall/js`
- Identity verified through application handshake (expectedPeerId from context)

## Post-Completion

**Manual verification:**
- Test with real domain SSL certificate against a running node
- Test browser client connectivity (from web app)
- Performance comparison with QUIC transport

**External system updates:**
- `any-sync-node` — add WebTransport config section and register transport component
- `any-sync-filenode` — same if file transfer over WebTransport is needed
- Deployment configs — open UDP port for HTTP/3, provision SSL certificates
