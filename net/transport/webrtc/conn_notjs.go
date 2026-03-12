//go:build !js

package webrtc

import pionwebrtc "github.com/pion/webrtc/v4"

func clearDCHandlers(_ *pionwebrtc.DataChannel) {}
func clearPCHandlers(_ *pionwebrtc.PeerConnection) {}
