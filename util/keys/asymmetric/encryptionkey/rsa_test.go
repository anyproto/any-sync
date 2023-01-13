package encryptionkey

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDeriveRSAKePair(t *testing.T) {
	privKey1, _, err := DeriveRSAKePair(4096, []byte("test seed"))
	require.NoError(t, err)

	privKey2, _, err := DeriveRSAKePair(4096, []byte("test seed"))
	require.NoError(t, err)
	data := []byte("test data")

	encryped, err := privKey1.GetPublic().Encrypt(data)
	require.NoError(t, err)

	decrypted, err := privKey2.Decrypt(encryped)
	require.NoError(t, err)

	assert.Equal(t, data, decrypted)
}
