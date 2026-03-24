package webtransport

import (
	"net"

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

// wtAddr implements net.Addr for WebTransport connections.
type wtAddr struct {
	addr string
}

func (a wtAddr) Network() string { return "webtransport" }
func (a wtAddr) String() string  { return a.addr }

// Compile-time check.
var _ net.Addr = wtAddr{}
