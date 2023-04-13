package crypto

import (
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDerivedKey(t *testing.T) {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	key, err := DeriveSymmetricKey(seed, AnysyncSpacePath)
	require.NoError(t, err)
	_, err = rand.Read(seed)
	require.NoError(t, err)
	res, err := key.Encrypt(seed)
	require.NoError(t, err)
	dec, err := key.Decrypt(res)
	require.NoError(t, err)
	require.Equal(t, seed, dec)
}
