package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAESKey_DecryptReuse(t *testing.T) {
	key := NewAES()
	messages := [][]byte{
		[]byte("hello world"),
		[]byte("short"),
		[]byte("a much longer message that should test buffer growth behavior properly"),
		[]byte("back to short"),
	}

	var buf []byte
	for _, msg := range messages {
		encrypted, err := key.Encrypt(msg)
		require.NoError(t, err)

		buf, err = key.DecryptReuse(buf, encrypted)
		require.NoError(t, err)
		require.Equal(t, msg, buf)

		// verify matches regular Decrypt
		plain, err := key.Decrypt(encrypted)
		require.NoError(t, err)
		require.Equal(t, plain, buf)
	}
}

func TestAESKey_DecryptReuse_NilDst(t *testing.T) {
	key := NewAES()
	msg := []byte("test message")
	encrypted, err := key.Encrypt(msg)
	require.NoError(t, err)

	result, err := key.DecryptReuse(nil, encrypted)
	require.NoError(t, err)
	require.Equal(t, msg, result)
}

func TestAESKey_DecryptReuse_BufferReuse(t *testing.T) {
	key := NewAES()
	msg := make([]byte, 256)
	_, err := rand.Read(msg)
	require.NoError(t, err)
	encrypted, err := key.Encrypt(msg)
	require.NoError(t, err)

	// first call allocates
	buf, err := key.DecryptReuse(nil, encrypted)
	require.NoError(t, err)
	firstPtr := &buf[:cap(buf)][cap(buf)-1]

	// second call with same-size message should reuse underlying array
	buf, err = key.DecryptReuse(buf, encrypted)
	require.NoError(t, err)
	secondPtr := &buf[:cap(buf)][cap(buf)-1]

	require.Equal(t, firstPtr, secondPtr, "expected buffer reuse")
}

func BenchmarkAESKey_Decrypt(b *testing.B) {
	key := NewAES()
	msg := make([]byte, 1024)
	rand.Read(msg)
	encrypted, _ := key.Encrypt(msg)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = key.Decrypt(encrypted)
	}
}

func BenchmarkAESKey_DecryptReuse(b *testing.B) {
	key := NewAES()
	msg := make([]byte, 1024)
	rand.Read(msg)
	encrypted, _ := key.Encrypt(msg)

	var buf []byte
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf, _ = key.DecryptReuse(buf, encrypted)
	}
}
