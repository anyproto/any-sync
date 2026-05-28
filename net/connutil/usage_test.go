package connutil

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastUsageConn_Bytes(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	luc := NewLastUsageConn(a)

	payload := []byte("hello, peer")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, len(payload))
		_, err := b.Read(buf)
		require.NoError(t, err)
		_, err = b.Write(buf)
		require.NoError(t, err)
	}()

	n, err := luc.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)

	buf := make([]byte, len(payload))
	rn, err := luc.Read(buf)
	require.NoError(t, err)
	require.Equal(t, len(payload), rn)

	wg.Wait()

	assert.Equal(t, int64(len(payload)), luc.BytesWritten())
	assert.Equal(t, int64(len(payload)), luc.BytesRead())
}

func TestLastUsageConn_BytesConcurrent(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	luc := NewLastUsageConn(a)

	const n = 100
	const size = 16
	payload := make([]byte, size)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, size)
		for i := 0; i < n; i++ {
			if _, err := b.Read(buf); err != nil {
				return
			}
			if _, err := b.Write(buf); err != nil {
				return
			}
		}
	}()

	for i := 0; i < n; i++ {
		_, err := luc.Write(payload)
		require.NoError(t, err)
		buf := make([]byte, size)
		_, err = luc.Read(buf)
		require.NoError(t, err)
	}
	wg.Wait()

	assert.Equal(t, int64(n*size), luc.BytesWritten())
	assert.Equal(t, int64(n*size), luc.BytesRead())
}
