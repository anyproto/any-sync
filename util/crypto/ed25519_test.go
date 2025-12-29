package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_EncryptDecrypt(t *testing.T) {
	privKey, pubKey, err := GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	msg := make([]byte, 32000)
	_, err = rand.Read(msg)
	require.NoError(t, err)
	enc, err := pubKey.Encrypt(msg)
	require.NoError(t, err)
	dec, err := privKey.Decrypt(enc)
	require.NoError(t, err)
	require.NotEqual(t, enc, dec)
	require.Equal(t, dec, msg)
}

func Test_SignVerify(t *testing.T) {
	privKey, pubKey, err := GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	msg := make([]byte, 32000)
	_, err = rand.Read(msg)
	sign, err := privKey.Sign(msg)
	require.NoError(t, err)
	res, err := pubKey.Verify(msg, sign)
	require.NoError(t, err)
	require.True(t, res)
}

func TestEd25519PublicKeyToCurve25519(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		_, err := Ed25519PublicKeyToCurve25519(pub)
		require.NoError(t, err)
	})
	t.Run("returns errors for arbitary bytes", func(t *testing.T) {
		pub := []byte{0, 1, 1, 0}
		_, err := Ed25519PublicKeyToCurve25519(pub)
		require.Error(t, err)

		pub = []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
		_, err = Ed25519PublicKeyToCurve25519(pub)
		require.Error(t, err)

	})

}

func Test_InvalidKey(t *testing.T) {
	corruptedKey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
	t.Run("decrypt", func(t *testing.T) {
		_, priv, _ := ed25519.GenerateKey(rand.Reader)
		withCorruptedPub := append(priv[:32], corruptedKey...)
		assert.Equal(t, 64, len(withCorruptedPub))

		corruptedPriv := NewEd25519PrivKey(withCorruptedPub)
		_, err := corruptedPriv.Decrypt([]byte{1, 2, 3, 4, 5, 6})
		require.Error(t, err)
	})

	t.Run("encrypt", func(t *testing.T) {
		corruptedPub := NewEd25519PubKey(corruptedKey)
		_, err := corruptedPub.Encrypt([]byte{1, 2, 3, 4, 5, 6})
		require.Error(t, err)

	})

}
