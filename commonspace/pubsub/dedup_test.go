package pubsub

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func testMsgId(n int) []byte {
	id := make([]byte, msgIdLen)
	binary.LittleEndian.PutUint64(id, uint64(n))
	return id
}

func TestDedupSeen(t *testing.T) {
	d := newMsgIdDedup(4)
	require.False(t, d.seen(testMsgId(1)))
	require.True(t, d.seen(testMsgId(1)))
	require.False(t, d.seen(testMsgId(2)))
	require.True(t, d.seen(testMsgId(2)))
}

func TestDedupEviction(t *testing.T) {
	d := newMsgIdDedup(4)
	for i := 1; i <= 4; i++ {
		require.False(t, d.seen(testMsgId(i)))
	}
	// adding a fifth evicts the oldest (1)
	require.False(t, d.seen(testMsgId(5)))
	require.False(t, d.seen(testMsgId(1)), "oldest id should have been evicted")
	// 1 was re-recorded above, evicting 2
	require.True(t, d.seen(testMsgId(5)))
	require.False(t, d.seen(testMsgId(2)))
}

func TestDedupInvalidLength(t *testing.T) {
	d := newMsgIdDedup(4)
	require.False(t, d.seen([]byte("short")))
	require.False(t, d.seen([]byte("short")), "invalid ids are never recorded")
}
