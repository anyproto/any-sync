package webtransport

import (
	_ "github.com/quic-go/webtransport-go"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/transport"
)

const CName = "net.transport.webtransport"

// WebTransport is the interface for the WebTransport transport component.
// On native (non-WASM) platforms it acts as both server and client.
// On WASM it is Dial-only.
type WebTransport interface {
	transport.Transport
	app.ComponentRunnable
}
