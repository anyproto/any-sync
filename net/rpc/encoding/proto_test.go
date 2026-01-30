package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type mockProtoMessage struct {
	data []byte
}

func (m *mockProtoMessage) SizeVT() int {
	return len(m.data)
}

func (m *mockProtoMessage) MarshalVT() ([]byte, error) {
	return m.data, nil
}

func (m *mockProtoMessage) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	copy(dAtA, m.data)
	return len(m.data), nil
}

func (m *mockProtoMessage) UnmarshalVT(dAtA []byte) error {
	m.data = append([]byte(nil), dAtA...)
	return nil
}

func (m *mockProtoMessage) ProtoReflect() protoreflect.Message {
	return nil
}

func TestProtoEncoding_MarshalAppend_ReuseLimit(t *testing.T) {
	enc := protoEncoding{}
	msg := &mockProtoMessage{data: []byte("small message")}

	t.Run("reuse small buffer", func(t *testing.T) {
		buf := make([]byte, 0, 1024)
		ptr := &buf[0:1][0]
		res, err := enc.MarshalAppend(buf, msg)
		require.NoError(t, err)
		assert.Equal(t, ptr, &res[0:1][0], "Should reuse the same underlying array for small buffers")
	})

	t.Run("do not reuse large buffer", func(t *testing.T) {
		buf := make([]byte, 0, maxBufSizeToReuse+1)
		ptr := &buf[0:1][0]
		res, err := enc.MarshalAppend(buf, msg)
		require.NoError(t, err)
		assert.NotEqual(t, ptr, &res[0:1][0], "Should NOT reuse the same underlying array for large buffers")
	})
}
