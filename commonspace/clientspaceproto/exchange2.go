package clientspaceproto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"sort"

	"golang.org/x/crypto/hkdf"

	"github.com/anyproto/any-sync/util/crypto"
)

// SpaceExchangeV2 token scheme.
//
// Peers never send space ids. Instead, for every space a peer holds it sends
// HMAC-SHA256(discoveryKey, nonce || label || callerPeerId || responderPeerId),
// where discoveryKey is derived from the space's first ACL read key — a stable
// secret available to every member. A matching token therefore simultaneously
// identifies a shared space and proves the sender's membership in it: strangers
// on the LAN see values indistinguishable from random, cannot correlate them
// across sessions (fresh nonce each run), and cannot claim spaces they are not
// members of. Spaces whose ACL state is not available locally are skipped.
//
// The responder answers only for the intersection, keying its tokens by the
// caller's nonce (challenge-response), so a response cannot be precomputed or
// replayed. Both request and response tokens are bound to the ordered pair of
// peer ids, so tokens observed on one connection are useless on another.

const (
	// NonceSizeV2 is the required size of SpaceExchangeV2Request.nonce
	NonceSizeV2 = 32
	// TokenSizeV2 is the size of every element of spaceTokens
	TokenSizeV2 = sha256.Size
	// MaxTokensV2 bounds spaceTokens in a request; handlers must reject bigger lists
	MaxTokensV2 = 4096
)

// tokenBucketsV2 are the sizes request token lists are padded to, hiding the real space count
var tokenBucketsV2 = []int{16, 32, 64, 128, 256}

var (
	labelRequestV2  = []byte("any-sync:space-exchange:v2:req")
	labelResponseV2 = []byte("any-sync:space-exchange:v2:resp")
	discoveryInfoV2 = []byte("any-sync:space-exchange:v2:discovery-key")
)

var ErrInvalidNonce = errors.New("space exchange v2: invalid nonce size")

// DeriveDiscoveryKey derives the per-space LAN discovery key from the first ACL read key.
// The first read key never rotates, so discovery keeps working for members whose local
// ACL copy is behind; former members retain it, but downstream sync still enforces the
// current ACL, so they can at most learn that a peer holds the space.
func DeriveDiscoveryKey(firstReadKey crypto.SymKey, spaceId string) ([]byte, error) {
	ikm, err := firstReadKey.Raw()
	if err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err = io.ReadFull(hkdf.New(sha256.New, ikm, []byte(spaceId), discoveryInfoV2), key); err != nil {
		return nil, err
	}
	return key, nil
}

// NewNonceV2 returns a fresh random nonce for a SpaceExchangeV2Request
func NewNonceV2() ([]byte, error) {
	nonce := make([]byte, NonceSizeV2)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

// RequestTokenV2 computes the caller's token for one space
func RequestTokenV2(discoveryKey, nonce []byte, callerPeerId, responderPeerId string) []byte {
	return tokenV2(discoveryKey, labelRequestV2, nonce, callerPeerId, responderPeerId)
}

// ResponseTokenV2 computes the responder's proof token for one intersecting space,
// keyed by the caller's nonce
func ResponseTokenV2(discoveryKey, nonce []byte, callerPeerId, responderPeerId string) []byte {
	return tokenV2(discoveryKey, labelResponseV2, nonce, callerPeerId, responderPeerId)
}

func tokenV2(discoveryKey, label, nonce []byte, callerPeerId, responderPeerId string) []byte {
	mac := hmac.New(sha256.New, discoveryKey)
	writeField(mac, label)
	writeField(mac, nonce)
	writeField(mac, []byte(callerPeerId))
	writeField(mac, []byte(responderPeerId))
	return mac.Sum(nil)
}

// writeField length-prefixes every field so variable-length peer ids cannot
// produce colliding concatenations
func writeField(w io.Writer, field []byte) {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(field)))
	_, _ = w.Write(lenBuf[:])
	_, _ = w.Write(field)
}

// PadTokensV2 pads tokens with random fillers up to the next bucket size (beyond the
// largest bucket — up to the next multiple of it) and sorts the result. Sorting is
// required: without it the position of real tokens would reveal the true space count
// the padding is meant to hide.
func PadTokensV2(tokens [][]byte) ([][]byte, error) {
	maxBucket := tokenBucketsV2[len(tokenBucketsV2)-1]
	target := (len(tokens) + maxBucket - 1) / maxBucket * maxBucket
	for _, bucket := range tokenBucketsV2 {
		if len(tokens) <= bucket {
			target = bucket
			break
		}
	}
	padded := make([][]byte, 0, target)
	padded = append(padded, tokens...)
	for len(padded) < target {
		filler := make([]byte, TokenSizeV2)
		if _, err := rand.Read(filler); err != nil {
			return nil, err
		}
		padded = append(padded, filler)
	}
	sort.Slice(padded, func(i, j int) bool {
		return string(padded[i]) < string(padded[j])
	})
	return padded, nil
}

// ContainsTokenV2 reports whether token is present in tokens, comparing in constant time
func ContainsTokenV2(tokens [][]byte, token []byte) bool {
	var found bool
	for _, t := range tokens {
		if hmac.Equal(t, token) {
			found = true
		}
	}
	return found
}
