package crypto

import (
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_EncryptDecrypt(t *testing.T) {
	privKey, pubKey, err := GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	msg := make([]byte, 32768)
	_, err = rand.Read(msg)
	require.NoError(t, err)
	enc, err := pubKey.Encrypt(msg)
	require.NoError(t, err)
	dec, err := privKey.Decrypt(enc)
	require.NoError(t, err)
	require.Equal(t, dec, msg)
}
