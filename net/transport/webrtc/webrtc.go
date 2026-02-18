package webrtc

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/transport"
)

const CName = "net.transport.webrtc"

// handshakeDCPrefix is the label prefix for the handshake DataChannel.
// The full label is "hs:<peerId>" so the server can learn the client's
// peerId before the application-level handshake.
const handshakeDCPrefix = "hs:"

// WebRTC is the interface for the WebRTC transport component.
// On native (non-WASM) platforms it acts as both server and client.
// On WASM it is Dial-only.
type WebRTC interface {
	transport.Transport
	app.ComponentRunnable
}
