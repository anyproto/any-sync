package clientspaceproto

import (
	"testing"

	"github.com/anyproto/any-sync/util/crypto"
)

// BenchmarkExchangeV2 measures a full exchange where both peers hold 1000 spaces,
// 500 of them shared. Discovery keys are precomputed once outside the loops —
// they are stable per space and meant to be cached; DeriveKeys measures that
// one-time cost separately. Matching uses a hash set, the right approach at this
// scale (ContainsTokenV2's linear scan is for small sets).
func BenchmarkExchangeV2(b *testing.B) {
	const (
		spacesPerPeer = 1000
		shared        = 500
	)
	caller, responder := "peerA", "peerB"

	newKeys := func(n int) []crypto.SymKey {
		keys := make([]crypto.SymKey, n)
		for i := range keys {
			key, err := crypto.NewRandomAES()
			if err != nil {
				b.Fatal(err)
			}
			keys[i] = key
		}
		return keys
	}
	deriveAll := func(keys []crypto.SymKey, ids []string) [][]byte {
		dks := make([][]byte, len(keys))
		for i := range keys {
			dk, err := DeriveDiscoveryKey(keys[i], ids[i])
			if err != nil {
				b.Fatal(err)
			}
			dks[i] = dk
		}
		return dks
	}

	sharedKeys := newKeys(shared)
	callerKeys := append(append([]crypto.SymKey{}, sharedKeys...), newKeys(spacesPerPeer-shared)...)
	responderKeys := append(append([]crypto.SymKey{}, sharedKeys...), newKeys(spacesPerPeer-shared)...)
	callerIds := make([]string, spacesPerPeer)
	responderIds := make([]string, spacesPerPeer)
	for i := range spacesPerPeer {
		if i < shared {
			callerIds[i] = "shared" + string(rune('0'+i%10)) + string(rune('a'+i/10%26)) + string(rune('a'+i/260))
			responderIds[i] = callerIds[i]
		} else {
			callerIds[i] = "caller-only-" + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676))
			responderIds[i] = "responder-only-" + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676))
		}
	}
	callerDks := deriveAll(callerKeys, callerIds)
	responderDks := deriveAll(responderKeys, responderIds)

	nonce, err := NewNonceV2()
	if err != nil {
		b.Fatal(err)
	}

	buildRequest := func() [][]byte {
		tokens := make([][]byte, len(callerDks))
		for i, dk := range callerDks {
			tokens[i] = RequestTokenV2(dk, nonce, caller, responder)
		}
		padded, err := PadTokensV2(tokens)
		if err != nil {
			b.Fatal(err)
		}
		return padded
	}
	handleRequest := func(reqTokens [][]byte) [][]byte {
		received := make(map[string]struct{}, len(reqTokens))
		for _, t := range reqTokens {
			received[string(t)] = struct{}{}
		}
		var respTokens [][]byte
		for _, dk := range responderDks {
			if _, ok := received[string(RequestTokenV2(dk, nonce, caller, responder))]; ok {
				respTokens = append(respTokens, ResponseTokenV2(dk, nonce, caller, responder))
			}
		}
		return respTokens
	}
	verifyResponse := func(respTokens [][]byte) int {
		received := make(map[string]struct{}, len(respTokens))
		for _, t := range respTokens {
			received[string(t)] = struct{}{}
		}
		var matched int
		for _, dk := range callerDks {
			if _, ok := received[string(ResponseTokenV2(dk, nonce, caller, responder))]; ok {
				matched++
			}
		}
		return matched
	}

	reqTokens := buildRequest()
	respTokens := handleRequest(reqTokens)
	if got := verifyResponse(respTokens); got != shared {
		b.Fatalf("expected %d shared spaces, got %d", shared, got)
	}

	b.Run("DeriveKeys", func(b *testing.B) {
		for b.Loop() {
			deriveAll(callerKeys, callerIds)
		}
	})
	b.Run("CallerBuildRequest", func(b *testing.B) {
		for b.Loop() {
			buildRequest()
		}
	})
	b.Run("ResponderHandle", func(b *testing.B) {
		for b.Loop() {
			handleRequest(reqTokens)
		}
	})
	b.Run("CallerVerifyResponse", func(b *testing.B) {
		for b.Loop() {
			verifyResponse(respTokens)
		}
	})
	b.Run("FullRoundTrip", func(b *testing.B) {
		for b.Loop() {
			verifyResponse(handleRequest(buildRequest()))
		}
	})
}
