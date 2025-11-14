package crypto

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDerivePath = "m/SLIP-0021/anysync/test"

func TestGenerateSharedKey(t *testing.T) {
	privKeyA, pubKeyA, _ := GenerateEd25519Key(rand.Reader)
	privKeyB, pubKeyB, _ := GenerateEd25519Key(rand.Reader)

	sharedSkA, err := GenerateSharedKey(privKeyA, pubKeyB, testDerivePath)
	require.NoError(t, err)

	sharedSkB, err := GenerateSharedKey(privKeyB, pubKeyA, testDerivePath)
	require.NoError(t, err)

	t.Run("both keys, derived for A and for B are equal", func(t *testing.T) {
		assert.Equal(t, sharedSkA, sharedSkB)

	})
	t.Run("A and B decrypt results are consistent", func(t *testing.T) {
		spkA := sharedSkA.GetPublic()
		spkB := sharedSkB.GetPublic()

		msg := []byte{1, 0, 1, 0, 1}
		encryptedA, err := spkA.Encrypt(msg)
		require.NoError(t, err)
		encryptedB, err := spkB.Encrypt(msg)
		require.NoError(t, err)

		assert.NotEqual(t, encryptedA, encryptedB)

		decryptedA, err := sharedSkA.Decrypt(encryptedB)
		require.NoError(t, err)
		decryptedB, err := sharedSkB.Decrypt(encryptedA)
		require.NoError(t, err)

		assert.Equal(t, decryptedA, decryptedB)
		assert.Equal(t, decryptedA, msg)

	})
	t.Run("C, generated with different path is different", func(t *testing.T) {
		spkA := sharedSkA.GetPublic()

		msg := []byte{1, 0, 1, 0, 1}
		encryptedA, err := spkA.Encrypt(msg)
		require.NoError(t, err)

		sharedSkC, err := GenerateSharedKey(privKeyB, pubKeyA, fmt.Sprintf("%s-two", testDerivePath))
		require.NoError(t, err)
		_, err = sharedSkC.Decrypt(encryptedA)
		require.Error(t, err)
	})
}
