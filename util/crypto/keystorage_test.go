package crypto

import (
	"crypto/rand"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestKeyStorage_PubKeyFromProto(t *testing.T) {
	st := NewKeyStorage().(*keyStorage)
	_, pubKey, err := GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		marshalled, err := pubKey.Marshall()
		require.NoError(t, err)
		pk, err := st.PubKeyFromProto(marshalled)
		require.NoError(t, err)
		require.Equal(t, pk.Storage(), pubKey.Storage())
	}
	require.Equal(t, 1, len(st.keys))
}
