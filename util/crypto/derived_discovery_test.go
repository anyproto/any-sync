package crypto

import (
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
