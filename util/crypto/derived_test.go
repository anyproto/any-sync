package crypto

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDerivedKey(t *testing.T) {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	key, err := DeriveSymmetricKey(seed, AnysyncSpacePath)
	require.NoError(t, err)
	_, err = rand.Read(seed)
	require.NoError(t, err)
	res, err := key.Encrypt(seed)
	require.NoError(t, err)
	dec, err := key.Decrypt(res)
	require.NoError(t, err)
	require.Equal(t, seed, dec)
}

func TestKeyDeriver_MatchesDeriveSymmetricKey(t *testing.T) {
	for i := 0; i < 10; i++ {
		seed := make([]byte, 32)
		_, err := rand.Read(seed)
		require.NoError(t, err)

		cid := fmt.Sprintf("bafytest%d", i)
		path := fmt.Sprintf(AnysyncTreePath, cid)

		expected, err := DeriveSymmetricKey(seed, path)
		require.NoError(t, err)

		deriver := NewKeyDeriver(path)
		got, err := deriver.DeriveKey(seed)
		require.NoError(t, err)

		expectedRaw, _ := expected.Raw()
		gotRaw, _ := got.Raw()
		require.Equal(t, expectedRaw, gotRaw, "seed %d: keys must match", i)
	}
}

func BenchmarkDeriveSymmetricKey(b *testing.B) {
	seed := make([]byte, 32)
	rand.Read(seed)
	path := fmt.Sprintf(AnysyncTreePath, "bafybench123")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DeriveSymmetricKey(seed, path)
	}
}

func BenchmarkKeyDeriver(b *testing.B) {
	seed := make([]byte, 32)
	rand.Read(seed)
	path := fmt.Sprintf(AnysyncTreePath, "bafybench123")
	deriver := NewKeyDeriver(path)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = deriver.DeriveKey(seed)
	}
}

func TestKeyDeriver_ReuseAcrossSeeds(t *testing.T) {
	path := fmt.Sprintf(AnysyncTreePath, "bafyreuse")
	deriver := NewKeyDeriver(path)

	// Derive with different seeds, ensure results differ and match DeriveSymmetricKey
	seeds := make([][]byte, 5)
	for i := range seeds {
		seeds[i] = make([]byte, 32)
		_, err := rand.Read(seeds[i])
		require.NoError(t, err)
	}

	seen := make(map[string]bool)
	for _, seed := range seeds {
		expected, err := DeriveSymmetricKey(seed, path)
		require.NoError(t, err)
		got, err := deriver.DeriveKey(seed)
		require.NoError(t, err)

		expectedRaw, _ := expected.Raw()
		gotRaw, _ := got.Raw()
		require.Equal(t, expectedRaw, gotRaw)

		key := string(gotRaw)
		require.False(t, seen[key], "derived keys should be unique per seed")
		seen[key] = true
	}
}
