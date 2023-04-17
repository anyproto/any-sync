package strkey

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDecode(t *testing.T) {
	key := "ABCw4rFBR7qU2HGzHwnKLYo9mMRcjGhFK28gSy58RKc5feqz"
	res, err := Decode(AccountAddressVersionByte, key)
	require.NoError(t, err)
	str, err := Encode(AccountAddressVersionByte, res)
	require.NoError(t, err)
	assert.Equal(t, key, str)
}
