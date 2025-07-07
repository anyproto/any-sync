package encoding

import (
	"bytes"
	"encoding/binary"
	"fmt"
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

func TestSnappyEncoding_MarshalAppend(t *testing.T) {
	t.Run("append to existing data", func(t *testing.T) {
		msg := &message{data: []byte("0000000000000000000000000000")}
		enc := &snappyEncoding{}
		buf := []byte{1, 2, 3}
		data, err := enc.MarshalAppend(buf, msg)
		require.NoError(t, err)
		msg2 := &message{}
		require.NoError(t, enc.Unmarshal(data[3:], msg2))
		assert.Equal(t, msg, msg2)
	})
	t.Run("reuse buf", func(t *testing.T) {
		var err error
		enc := &snappyEncoding{}
		buf := make([]byte, 1024)
		for i := range 5 {
			msg := &message{data: []byte(fmt.Sprintf("xxxxxxxxxxx%d", i))}
			buf, err = enc.MarshalAppend(buf[:0], msg)
			require.NoError(t, err)
			msg2 := &message{}
			require.NoError(t, enc.Unmarshal(buf, msg2))
			assert.Equal(t, msg.data, msg2.data)
		}

	})
}

func BenchmarkSnappyEncoding_Marshal(b *testing.B) {
	var buf []byte
	var enc = &snappyEncoding{}
	var msg = &message{data: make([]byte, 8)}
	var unmarshalMsg = &message{}
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for n := range b.N {
		binary.LittleEndian.PutUint64(msg.data, uint64(n)+1)
		buf, err = enc.MarshalAppend(buf[:0], msg)
		if err != nil {
			b.Fatal(err)
		}
		if err = enc.Unmarshal(buf, unmarshalMsg); err != nil {
			b.Fatal(err)
		}
		if !bytes.Equal(msg.data, unmarshalMsg.data) {
			b.Fatalf("expected %v, got %v", msg.data, unmarshalMsg.data)
		}
	}
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
