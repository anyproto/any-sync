package crypto

import (
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestMnemonic(t *testing.T) {
	phrase, err := NewMnemonicGenerator().WithWordCount(12)
	require.NoError(t, err)
	parts := strings.Split(string(phrase), " ")
	require.Equal(t, 12, len(parts))
	key, err := phrase.DeriveEd25519Key(0)
	require.NoError(t, err)
	bytes := make([]byte, 64)
	_, err = rand.Read(bytes)
	require.NoError(t, err)
	sign, err := key.Sign(bytes)
	require.NoError(t, err)
	res, err := key.GetPublic().Verify(bytes, sign)
	require.NoError(t, err)
	require.True(t, res)
}
