package encoding

import (
	"testing"

	"github.com/anyproto/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnappyEncoding_Marshal(t *testing.T) {
	msg := &message{data: []byte("0000000000000000000000000000")}
	enc := &snappyEncoding{}
	data, err := enc.Marshal(msg)
	require.NoError(t, err)
	assert.NotEqual(t, msg.data, data)
	msg2 := &message{}
	require.NoError(t, enc.Unmarshal(data, msg2))
	assert.Equal(t, msg, msg2)
	t.Log(len(data), len(msg.data))
}
