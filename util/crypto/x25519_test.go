package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SharedKeyEqual(t *testing.T) {
	privKeyA, pubKeyA, _ := GenerateEd25519Key(rand.Reader)
	rawSkA, _ := privKeyA.Raw()
	rawPkA, _ := pubKeyA.Raw()
	privKeyB, pubKeyB, _ := GenerateEd25519Key(rand.Reader)
	rawSkB, _ := privKeyB.Raw()
	rawPkB, _ := pubKeyB.Raw()

	sharedSkA, sharedPkA, err := GenerateSharedKey(ed25519.PrivateKey(rawSkA), ed25519.PublicKey(rawPkA), rawPkB)
	require.NoError(t, err)

	sharedSkB, sharedPkB, err := GenerateSharedKey(ed25519.PrivateKey(rawSkB), ed25519.PublicKey(rawPkB), rawPkA)
	require.NoError(t, err)

	assert.Equal(t, sharedSkA, sharedSkB)
	assert.Equal(t, sharedPkA, sharedPkB)
}

func Test_SharedKeyEncryptDecrypt(t *testing.T) {
	privKeyA, pubKeyA, _ := GenerateEd25519Key(rand.Reader)
	rawSkA, _ := privKeyA.Raw()
	rawPkA, _ := pubKeyA.Raw()
	privKeyB, pubKeyB, _ := GenerateEd25519Key(rand.Reader)
	rawSkB, _ := privKeyB.Raw()
	rawPkB, _ := pubKeyB.Raw()

	sharedSkA, sharedPkA, err := GenerateSharedKey(ed25519.PrivateKey(rawSkA), ed25519.PublicKey(rawPkA), rawPkB)
	require.NoError(t, err)

	sharedSkB, sharedPkB, err := GenerateSharedKey(ed25519.PrivateKey(rawSkB), ed25519.PublicKey(rawPkB), rawPkA)
	require.NoError(t, err)

	pkA := NewEd25519PubKey(sharedPkA)
	skA := NewEd25519PrivKey(sharedSkA)
	pkB := NewEd25519PubKey(sharedPkB)
	skB := NewEd25519PrivKey(sharedSkB)

	msg := []byte{1, 0, 1, 0, 1}
	encryptedA, err := pkA.Encrypt(msg)
	require.NoError(t, err)
	encryptedB, err := pkB.Encrypt(msg)
	require.NoError(t, err)

	assert.NotEqual(t, encryptedA, encryptedB)

	decryptedA, err := skA.Decrypt(encryptedB)
	require.NoError(t, err)
	decryptedB, err := skB.Decrypt(encryptedA)
	require.NoError(t, err)

	assert.Equal(t, decryptedA, decryptedB)
	assert.Equal(t, decryptedA, msg)

}
