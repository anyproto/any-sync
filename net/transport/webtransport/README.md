# WebTransport Transport

WebTransport is an optional transport for any-sync that runs over HTTP/3 (QUIC). It supplements the primary yamux (TCP) and QUIC transports with a protocol that browsers can use natively via the [WebTransport API](https://developer.mozilla.org/en-US/docs/Web/API/WebTransport).

## When to use

- **Browser-to-node** -- the main use case. Browsers (WASM clients) can connect directly to any-sync nodes without a WebSocket proxy.
- **Node-to-node** -- supported on native platforms, but yamux/QUIC are generally preferred because they are more mature and don't require a separate TLS certificate.

## Architecture

The package implements `transport.Transport` and `app.ComponentRunnable` behind the `WebTransport` interface. It is registered as an optional component under the name `net.transport.webtransport`. During `Init`, `peerservice` checks whether the component is present and, if so, wires it into the transport selection logic alongside yamux and QUIC.

WebTransport addresses use the `webtransport://` scheme. When `peerservice.Dial` resolves addresses for a peer, it tries each transport's addresses in preference order (yamux or QUIC first, then WebTransport).

### Platform builds

| Build | Server | Client | Implementation |
|-------|--------|--------|----------------|
| Native (`!js`) | Yes | Yes | `webtransport_native.go`, `conn.go` -- uses `github.com/quic-go/webtransport-go` |
| WASM (`js`) | No | Yes | `webtransport_js.go`, `conn_js.go` -- uses the browser `WebTransport` constructor via `syscall/js` |

Both builds produce `New() WebTransport` with the same interface; the WASM build's `Run` and `Close` are no-ops.

## Connection flow

### Native server

1. **TLS setup** -- The server loads the TLS certificate and key from disk on every new connection (`GetCertificate` callback), so certificate rotation (e.g. Let's Encrypt renewals) does not require a restart.
2. **QUIC listener** -- A UDP socket is opened for each `listenAddrs` entry. An HTTP/3 server runs on top of QUIC.
3. **WebTransport upgrade** -- An HTTP handler at the configured `path` (default `/webtransport`) upgrades the connection to a WebTransport session. CORS headers are set to allow browser origins.
4. **PeerId extraction** -- The server reads the `peerId` query parameter from the upgrade URL. If missing, the session is rejected.
5. **Handshake** -- The server accepts the first bidirectional stream and runs `secureservice.HandshakeInbound` over it. This exchanges `ProtoVersion`, verifies the remote identity, and populates the connection context with the authenticated peerId, identity, and protocol version.
6. **Accept** -- On success the handshake stream is closed and the `wtMultiConn` is passed to the registered `Accepter` (typically `peerservice`), which creates a `peer.Peer` and adds it to the pool.

### Native client

1. The caller places the expected peerId in context via `peer.CtxWithExpectedPeerId`.
2. `Dial` constructs a URL: `https://<addr><path>?peerId=<localPeerId>` and opens a QUIC+WebTransport session (`InsecureSkipVerify: true` -- identity is verified at the handshake layer, not TLS).
3. A bidirectional stream is opened and `secureservice.HandshakeOutbound` runs over it, verifying the remote peer's identity matches the expected peerId.
4. On success the handshake stream is closed and a `wtMultiConn` wrapping the session is returned.

### WASM / browser client

1. Same context requirement: `peer.CtxWithExpectedPeerId` must be set.
2. `Dial` creates a `new WebTransport(url)` JS object and awaits its `ready` promise.
3. A bidirectional stream is created via `createBidirectionalStream()` and wrapped as a `jsStream` (implements `net.Conn`).
4. `secureservice.HandshakeOutbound` runs over the stream.
5. On success the stream is closed and a `jsMultiConn` wrapping the JS WebTransport object is returned. The `jsMultiConn` watches the `closed` promise to detect disconnection.

## Handshake and PeerId

WebTransport does not use libp2p's TLS identity mechanism (unlike yamux and QUIC), so the remote peerId cannot be extracted from the TLS certificate. Instead:

- The **dialing side** passes its own `peerId` as a URL query parameter (`?peerId=...`).
- The **receiving side** reads it from the URL and passes it to `HandshakeInbound`.
- Both sides run the any-sync secure handshake protocol over a dedicated bidirectional stream. This exchanges `ProtoVersion` numbers, cryptographic identities, and signatures to authenticate both peers.
- `CtxWithExpectedPeerId` is required in the dial context so that `HandshakeOutbound` can verify the remote peer's identity matches. If the expected peerId is missing from context, `Dial` returns an error immediately.

## Configuration

The `Config` struct is read from the application config via the `configGetter` interface (`GetWebTransport() Config`).

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

| Field | Default | Description |
|-------|---------|-------------|
| `listenAddrs` | *(none)* | UDP addresses for the QUIC/HTTP3 listener. Server-only; omit on dial-only nodes. |
| `path` | `/webtransport` | URL path for the WebTransport upgrade endpoint. |
| `certFile` | *(required for server)* | Path to a PEM-encoded TLS certificate file. Re-read on each connection. |
| `keyFile` | *(required for server)* | Path to a PEM-encoded TLS private key file. Re-read on each connection. |
| `writeTimeoutSec` | `0` (disabled) | Per-write deadline on streams. |
| `closeTimeoutSec` | `5` | Timeout for graceful session close before giving up. |
| `dialTimeoutSec` | `30` | Timeout for outbound dial and handshake. Also used as the accept-handshake timeout on the server. |
| `maxStreams` | `128` | Maximum concurrent incoming QUIC streams per session. |

## Key files

| File | Purpose |
|------|---------|
| `webtransport.go` | Shared interface, `CName`, `wtAddr` type |
| `webtransport_native.go` | Native server + client implementation |
| `webtransport_js.go` | WASM/browser client implementation |
| `conn.go` | Native `wtMultiConn` and `wtNetConn` (wraps `webtransport.Stream`) |
| `conn_js.go` | WASM `jsMultiConn` and `jsStream` (wraps browser bidirectional streams) |
| `config.go` | `Config` struct and `configGetter` interface |
