package crypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveDiscoveryKeys(t *testing.T) {
	identity, _, err := GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	sign1, enc1, err := DeriveDiscoveryKeys(identity)
	require.NoError(t, err)
	sign2, enc2, err := DeriveDiscoveryKeys(identity)
	require.NoError(t, err)

	// deterministic
	assert.True(t, sign1.Equals(sign2))
	assert.True(t, enc1.Equals(enc2))
	// never the identity key itself
	assert.False(t, sign1.Equals(identity))
	assert.NotEqual(t, identity.GetPublic().Account(), sign1.GetPublic().Account())

	other, _, err := GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	sign3, enc3, err := DeriveDiscoveryKeys(other)
	require.NoError(t, err)
	assert.False(t, sign1.Equals(sign3))
	assert.False(t, enc1.Equals(enc3))

	// the signing key is a usable ed25519 key
	sig, err := sign1.Sign([]byte("record"))
	require.NoError(t, err)
	ok, err := sign1.GetPublic().Verify([]byte("record"), sig)
	require.NoError(t, err)
	assert.True(t, ok)
}

// The derivation is frozen: published records are addressed by these keys.
func TestDeriveDiscoveryKeys_KnownAnswer(t *testing.T) {
	seed := bytes.Repeat([]byte{0x2a}, ed25519.SeedSize)
	identity := NewEd25519PrivKey(ed25519.NewKeyFromSeed(seed))
	require.Equal(t, "A6DTa5q8PmGHM8aLTSm7xdh47LQBLvFdc2g3PpXUkRiXBk9R", identity.GetPublic().Account())

	sign, enc, err := DeriveDiscoveryKeys(identity)
	require.NoError(t, err)
	pub, err := sign.GetPublic().Raw()
	require.NoError(t, err)
	assert.Equal(t, "15aedadf32b1a8248022cf4d09425f62abb9b6109400798fbc2e99324a4ce556", hex.EncodeToString(pub))
	assert.Equal(t, "A68TUHXwkUgscvvEknmv8JiAuKbYjuL2iypdedMpfXTs17qU", sign.GetPublic().Account())
	encRaw, err := enc.Raw()
	require.NoError(t, err)
	assert.Equal(t, "e70b9f8823c6519b69bbeaf0ff200a508159aaecf42cb7f3e4064ce8656ef102", hex.EncodeToString(encRaw))
}
