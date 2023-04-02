package strkey

import (
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDecode(t *testing.T) {
	_, pubKey, err := signingkey.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	raw, _ := pubKey.Raw()
	str, err := Encode(NetworkAddressVersionByte, raw)
	require.NoError(t, err)
	res, err := Decode(NetworkAddressVersionByte, str)
	require.NoError(t, err)
	assert.Equal(t, raw, res)
}
