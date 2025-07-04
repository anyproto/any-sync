package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
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

type message struct {
	data []byte
}

func (m *message) SizeVT() (n int) {
	return len(m.data)
}

func (m *message) MarshalVT() (dAtA []byte, err error) {
	return m.data, nil
}

func (m *message) UnmarshalVT(dAtA []byte) error {
	m.data = dAtA
	return nil
}

func (m *message) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	copy(dAtA, m.data)
	return len(m.data), nil
}

func (m *message) ProtoReflect() protoreflect.Message {
	return nil
}
