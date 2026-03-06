package webrtc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const signalPath = "/webrtc/signal"

// buildSignalURL constructs the signaling URL from a WebRTC address.
// For path-based addresses (e.g. "example.com/signal/node") it uses https.
// For host:port addresses (e.g. "127.0.0.1:5301") it uses http.
func buildSignalURL(addr string) string {
	addr = strings.TrimRight(addr, "/")
	scheme := "http"
	if strings.Contains(addr, "/") {
		scheme = "https"
	}
	return scheme + "://" + addr + signalPath
}

// signalMessage carries an SDP offer or answer between client and server.
// No identity information travels through signaling — identity is verified
// via the DTLS handshake (certhash / libp2p TLS) and the application-level
// handshake on the first DataChannel.
type signalMessage struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"` // "offer" or "answer"
}

// signalExchange sends an SDP offer to the signaling endpoint and returns
// the answer.
func signalExchange(ctx context.Context, signalURL string, offer signalMessage) (answer signalMessage, err error) {
	body, err := json.Marshal(offer)
	if err != nil {
		return answer, fmt.Errorf("marshal offer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, signalURL, bytes.NewReader(body))
	if err != nil {
		return answer, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return answer, fmt.Errorf("signal request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return answer, fmt.Errorf("signal server returned %d: %s", resp.StatusCode, string(respBody))
	}

	if err = json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return answer, fmt.Errorf("decode answer: %w", err)
	}
	return answer, nil
}
