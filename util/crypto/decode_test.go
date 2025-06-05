package crypto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeNetworkId(t *testing.T) {
	_, pubKey, err := GenerateRandomEd25519KeyPair()
	require.NoError(t, err)

	networkId := pubKey.Network()
	require.Equal(t, uint8('N'), networkId[0])
	decodedKey, err := DecodeNetworkId(networkId)
	require.NoError(t, err)
	require.Equal(t, pubKey, decodedKey)
}
