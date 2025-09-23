package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SharedKeyEqual(t *testing.T) {
	privKeyA, pubKeyA, _ := GenerateEd25519Key(rand.Reader)
	privKeyB, pubKeyB, _ := GenerateEd25519Key(rand.Reader)

	sharedSkA, err := GenerateSharedKey(privKeyA, pubKeyB, "test")
	require.NoError(t, err)

	sharedSkB, err := GenerateSharedKey(privKeyB, pubKeyA, "test")
	require.NoError(t, err)

	assert.Equal(t, sharedSkA, sharedSkB)
}

func Test_SharedKeyEncryptDecrypt(t *testing.T) {
	privKeyA, pubKeyA, _ := GenerateEd25519Key(rand.Reader)
	privKeyB, pubKeyB, _ := GenerateEd25519Key(rand.Reader)

	sharedSkA, err := GenerateSharedKey(privKeyA, pubKeyB, "test")
	require.NoError(t, err)

	sharedSkB, err := GenerateSharedKey(privKeyB, pubKeyA, "test")
	require.NoError(t, err)

	pkA := sharedSkA.GetPublic()
	pkB := sharedSkB.GetPublic()

	msg := []byte{1, 0, 1, 0, 1}
	encryptedA, err := pkA.Encrypt(msg)
	require.NoError(t, err)
	encryptedB, err := pkB.Encrypt(msg)
	require.NoError(t, err)

	assert.NotEqual(t, encryptedA, encryptedB)

	decryptedA, err := sharedSkA.Decrypt(encryptedB)
	require.NoError(t, err)
	decryptedB, err := sharedSkB.Decrypt(encryptedA)
	require.NoError(t, err)

	assert.Equal(t, decryptedA, decryptedB)
	assert.Equal(t, decryptedA, msg)

	sharedSkC, err := GenerateSharedKey(privKeyB, pubKeyA, "test2")
	require.NoError(t, err)
	_, err = sharedSkC.Decrypt(encryptedA)
	require.Error(t, err)

}
