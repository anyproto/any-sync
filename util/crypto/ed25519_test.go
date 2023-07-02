package crypto

import (
	"crypto/rand"
	"testing"

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
