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

func TestUnmarshalEd25519PublicKey(t *testing.T) {
	t.Run("valid key", func(t *testing.T) {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		_, err = UnmarshalEd25519PublicKey(pub)
		require.NoError(t, err)
	})
	t.Run("wrong length", func(t *testing.T) {
		_, err := UnmarshalEd25519PublicKey([]byte{1, 2, 3})
		require.Error(t, err)
	})
	t.Run("32 bytes not on curve", func(t *testing.T) {
		// 32 bytes that pass length check but do not decode as a valid
		// Edwards curve point. Same pattern used by TestEd25519PublicKeyToCurve25519.
		badPoint := []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
		_, err := UnmarshalEd25519PublicKey(badPoint)
		require.Error(t, err)
	})
	t.Run("proto wrapper rejects non-curve point", func(t *testing.T) {
		badPoint := []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
		proto, err := NewEd25519PubKey(badPoint).Marshall()
		require.NoError(t, err)
		_, err = UnmarshalEd25519PublicKeyProto(proto)
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
