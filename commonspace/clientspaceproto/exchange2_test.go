package clientspaceproto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/util/crypto"
)

func TestDeriveDiscoveryKey(t *testing.T) {
	readKey, err := crypto.NewRandomAES()
	require.NoError(t, err)

	key1, err := DeriveDiscoveryKey(readKey, "space1")
	require.NoError(t, err)
	require.Len(t, key1, 32)

	// deterministic for the same inputs
	key1Again, err := DeriveDiscoveryKey(readKey, "space1")
	require.NoError(t, err)
	require.Equal(t, key1, key1Again)

	// different per space even with the same read key
	key2, err := DeriveDiscoveryKey(readKey, "space2")
	require.NoError(t, err)
	require.NotEqual(t, key1, key2)

	// raw read key never appears as the discovery key
	raw, err := readKey.Raw()
	require.NoError(t, err)
	require.NotEqual(t, raw, key1)
}

func TestTokensV2(t *testing.T) {
	readKey, err := crypto.NewRandomAES()
	require.NoError(t, err)
	dk, err := DeriveDiscoveryKey(readKey, "space1")
	require.NoError(t, err)
	nonce, err := NewNonceV2()
	require.NoError(t, err)

	req := RequestTokenV2(dk, nonce, "peerA", "peerB")
	require.Len(t, req, TokenSizeV2)

	// request and response tokens must differ (domain separation)
	resp := ResponseTokenV2(dk, nonce, "peerA", "peerB")
	require.NotEqual(t, req, resp)

	// bound to the peer pair
	require.NotEqual(t, req, RequestTokenV2(dk, nonce, "peerA", "peerC"))
	require.NotEqual(t, req, RequestTokenV2(dk, nonce, "peerB", "peerA"))

	// bound to the nonce
	nonce2, err := NewNonceV2()
	require.NoError(t, err)
	require.NotEqual(t, req, RequestTokenV2(dk, nonce2, "peerA", "peerB"))

	// length prefixing: shifting a boundary between peer ids must change the token
	require.NotEqual(t, RequestTokenV2(dk, nonce, "peerAp", "eerB"), RequestTokenV2(dk, nonce, "peerA", "peerB"))
}

func TestPadTokensV2(t *testing.T) {
	tokens := make([][]byte, 0, 20)
	for range 20 {
		tok, err := NewNonceV2()
		require.NoError(t, err)
		tokens = append(tokens, tok)
	}

	padded, err := PadTokensV2(tokens[:3])
	require.NoError(t, err)
	require.Len(t, padded, 16)

	padded, err = PadTokensV2(tokens[:16])
	require.NoError(t, err)
	require.Len(t, padded, 16)

	padded, err = PadTokensV2(tokens[:17])
	require.NoError(t, err)
	require.Len(t, padded, 32)

	// all real tokens survive padding
	for _, tok := range tokens[:17] {
		require.True(t, ContainsTokenV2(padded, tok))
	}

	// sorted, so real token positions don't leak the true count
	for i := 1; i < len(padded); i++ {
		require.LessOrEqual(t, string(padded[i-1]), string(padded[i]))
	}

	// beyond the largest bucket: next multiple of it
	big := make([][]byte, 300)
	for i := range big {
		big[i], err = NewNonceV2()
		require.NoError(t, err)
	}
	padded, err = PadTokensV2(big)
	require.NoError(t, err)
	require.Len(t, padded, 512)
}

// TestExchangeV2RoundTrip simulates a full exchange between two peers
func TestExchangeV2RoundTrip(t *testing.T) {
	shared1, err := crypto.NewRandomAES()
	require.NoError(t, err)
	shared2, err := crypto.NewRandomAES()
	require.NoError(t, err)
	callerOnly, err := crypto.NewRandomAES()
	require.NoError(t, err)
	responderOnly, err := crypto.NewRandomAES()
	require.NoError(t, err)

	type space struct {
		id      string
		readKey crypto.SymKey
	}
	callerSpaces := []space{{"shared1", shared1}, {"shared2", shared2}, {"callerOnly", callerOnly}}
	responderSpaces := []space{{"shared1", shared1}, {"shared2", shared2}, {"responderOnly", responderOnly}}
	const caller, responder = "peerA", "peerB"

	// caller builds the request
	nonce, err := NewNonceV2()
	require.NoError(t, err)
	var reqTokens [][]byte
	for _, sp := range callerSpaces {
		dk, err := DeriveDiscoveryKey(sp.readKey, sp.id)
		require.NoError(t, err)
		reqTokens = append(reqTokens, RequestTokenV2(dk, nonce, caller, responder))
	}
	reqTokens, err = PadTokensV2(reqTokens)
	require.NoError(t, err)

	// responder matches against its own spaces and answers for the intersection
	var respTokens [][]byte
	var responderIntersection []string
	for _, sp := range responderSpaces {
		dk, err := DeriveDiscoveryKey(sp.readKey, sp.id)
		require.NoError(t, err)
		if ContainsTokenV2(reqTokens, RequestTokenV2(dk, nonce, caller, responder)) {
			responderIntersection = append(responderIntersection, sp.id)
			respTokens = append(respTokens, ResponseTokenV2(dk, nonce, caller, responder))
		}
	}
	require.Equal(t, []string{"shared1", "shared2"}, responderIntersection)

	// caller verifies the response
	var callerIntersection []string
	for _, sp := range callerSpaces {
		dk, err := DeriveDiscoveryKey(sp.readKey, sp.id)
		require.NoError(t, err)
		if ContainsTokenV2(respTokens, ResponseTokenV2(dk, nonce, caller, responder)) {
			callerIntersection = append(callerIntersection, sp.id)
		}
	}
	require.Equal(t, []string{"shared1", "shared2"}, callerIntersection)

	// a peer that knows only the space id (no read key) gets no match
	wrongKey, err := crypto.NewRandomAES()
	require.NoError(t, err)
	dk, err := DeriveDiscoveryKey(wrongKey, "shared1")
	require.NoError(t, err)
	require.False(t, ContainsTokenV2(reqTokens, RequestTokenV2(dk, nonce, caller, responder)))
}
