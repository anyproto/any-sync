//go:build !js

package webrtc

import (
	"encoding/json"
	"io"
	"net/http"
)

// signalHandler is an http.Handler that processes SDP offers and returns answers.
type signalHandler struct {
	handleOffer func(offerSDP string) (answerSDP string, err error)
}

func (h *signalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var msg signalMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if msg.Type != "offer" || msg.SDP == "" {
		http.Error(w, "expected SDP offer", http.StatusBadRequest)
		return
	}

	answerSDP, err := h.handleOffer(msg.SDP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := signalMessage{
		SDP:  answerSDP,
		Type: "answer",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
