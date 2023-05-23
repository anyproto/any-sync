package crypto

import (
	"crypto/rand"
	"github.com/anyproto/go-slip10"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestMnemonic(t *testing.T) {
	phrase, err := NewMnemonicGenerator().WithWordCount(12)
	require.NoError(t, err)
	parts := strings.Split(string(phrase), " ")
	require.Equal(t, 12, len(parts))
	res, err := phrase.DeriveKeys(0)
	require.NoError(t, err)
	bytes := make([]byte, 64)
	_, err = rand.Read(bytes)
	require.NoError(t, err)

	// testing signing with keys
	for _, k := range []PrivKey{res.MasterKey, res.Identity, res.OldAccountKey} {
		sign, err := k.Sign(bytes)
		require.NoError(t, err)
		res, err := k.GetPublic().Verify(bytes, sign)
		require.NoError(t, err)
		require.True(t, res)
	}

	// testing derivation
	masterKey, err := genKey(res.MasterNode)
	require.NoError(t, err)
	require.True(t, res.MasterKey.Equals(masterKey))
	identityNode, err := res.MasterNode.Derive(slip10.FirstHardenedIndex)
	require.NoError(t, err)
	identityKey, err := genKey(identityNode)
	require.NoError(t, err)
	require.True(t, res.Identity.Equals(identityKey))
	oldAccountRes, err := phrase.deriveForPath(true, 0, anytypeAccountOldPrefix)
	require.NoError(t, err)
	require.True(t, res.OldAccountKey.Equals(oldAccountRes.MasterKey))
}
