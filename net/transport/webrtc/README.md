# WebRTC Transport for any-sync

WebRTC transport enables browser-based (WASM) clients to connect to any-sync nodes. It uses WebRTC DataChannels as the underlying transport, with SDP signaling handled over a plain HTTP endpoint on the server itself — no separate signaling server required.

## Architecture

```
WASM client (browser)                  Go server (any-sync node)
         |                                       |
         |--- POST /webrtc/signal {SDP offer} -->|  (1) HTTP signaling
         |<-- 200 OK {SDP answer} ---------------|
         |                                       |
         |======= WebRTC DataChannels ==========>|  (2) P2P data
         |                                       |
         |--- handshake DC: identity exchange -->|  (3) App-level auth
         |<-- verified peer context -------------|
```

1. The client creates an SDP offer and POSTs it to the server's `/webrtc/signal` endpoint
2. The server responds with an SDP answer; both sides establish a WebRTC connection
3. A handshake DataChannel verifies identity using `secureservice` (same as yamux/quic)

## Integration

### Server side (Go node)

Register the WebRTC transport component in your app:

```go
import "github.com/anyproto/any-sync/net/transport/webrtc"

app.Register(webrtc.New())
```

Your config struct must implement the `configGetter` interface:

```go
type configGetter interface {
    GetWebRTC() webrtc.Config
}
```

Config fields:

```yaml
webrtc:
  listenAddrs:
    - "0.0.0.0:8080"    # address for the HTTP signal endpoint
  signalPort: 0          # override port for signal endpoint (0 = same as listenAddrs)
  dialTimeoutSec: 30     # timeout for ICE gathering and connection setup
  closeTimeoutSec: 5     # graceful shutdown timeout
  writeTimeoutSec: 0     # write deadline on DataChannel streams
```

Required companion components (must already be registered):
- `secureservice` — cryptographic handshake
- `accountservice` — local peer identity

### Client side (WASM)

Same registration:

```go
import "github.com/anyproto/any-sync/net/transport/webrtc"

app.Register(webrtc.New())
```

Build with:

```bash
GOOS=js GOARCH=wasm go build -o app.wasm ./cmd/wasm
```

The WASM transport is dial-only (no `Run` / no listening). It uses the browser's native `RTCPeerConnection` via `pion/webrtc/v4`.

### Peer addresses

Use the `webrtc://` scheme in peer addresses:

```go
peerService.SetPeerAddrs("peer123", []string{
    "webrtc://node.example.com:8080",
})
```

The address points to the host and port where the signal HTTP endpoint is running.

## Transport optionality

As of this branch, **all transports (yamux, quic, webrtc) are optional** in `peerservice`. It uses `a.Component()` instead of `a.MustComponent()`, so only registered transports are active. This means:

- A WASM app can register **only** WebRTC — no panic from missing yamux/quic
- A traditional node can register all three — behavior is identical to before
- `peerservice.Dial()` builds the scheme priority list dynamically from whatever transports are registered

No code changes needed in existing nodes that don't use WebRTC — they continue to work as before.

## WASM build compatibility

The `commonfile/fileservice` package is excluded from WASM builds (`//go:build !js`) because it depends on `github.com/ipfs/boxo` which uses `syscall.O_NOFOLLOW`. If your WASM app needs file operations, use the `FileHandler` type directly with a WASM-compatible blockstore instead.

## Security

Identity is **not** exchanged during signaling. The SDP offer/answer only carries ICE candidates and DTLS fingerprints. Peer identity is verified through the application-level handshake on the first DataChannel, using the same `secureservice.HandshakeOutbound`/`HandshakeInbound` used by yamux and quic transports.
