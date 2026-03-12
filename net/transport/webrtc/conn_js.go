//go:build js

package webrtc

import (
	"syscall/js"

	pionwebrtc "github.com/pion/webrtc/v4"
)

// clearDCHandlers nulls out JS event handler properties on the underlying
// RTCDataChannel BEFORE pion's Close() releases the Go js.Func wrappers.
// Without this, the browser fires async events (e.g. 'close') after
// pion has already called js.Func.Release(), causing
// "call to released function" panics.
func clearDCHandlers(dc *pionwebrtc.DataChannel) {
	jsVal := dc.JSValue()
	null := js.Null()
	jsVal.Set("onclose", null)
	jsVal.Set("onclosing", null)
	jsVal.Set("onmessage", null)
	jsVal.Set("onopen", null)
	jsVal.Set("onerror", null)
	jsVal.Set("onbufferedamountlow", null)
}

// clearPCHandlers nulls out JS event handler properties on the underlying
// RTCPeerConnection before pion's Close() releases them.
func clearPCHandlers(pc *pionwebrtc.PeerConnection) {
	jsVal := pc.JSValue()
	null := js.Null()
	jsVal.Set("onconnectionstatechange", null)
	jsVal.Set("ondatachannel", null)
	jsVal.Set("onicecandidate", null)
	jsVal.Set("oniceconnectionstatechange", null)
	jsVal.Set("onicegatheringstatechange", null)
	jsVal.Set("onnegotiationneeded", null)
	jsVal.Set("onsignalingstatechange", null)
}
