package streampool

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/net/streampool/testservice"
)

func TestProtoEncoding(t *testing.T) {
	t.Run("not a proto err", func(t *testing.T) {
		_, err := EncodingProto.Marshal("string")
		assert.Error(t, err)
		err = EncodingProto.Unmarshal(nil, "sss")
		assert.Error(t, err)
	})
	t.Run("encode marshal", func(t *testing.T) {
		data, err := EncodingProto.Marshal(&testservice.StreamMessage{ReqData: "1"})
		require.NoError(t, err)
		msg := &testservice.StreamMessage{}
		require.NoError(t, EncodingProto.Unmarshal(data, msg))
		assert.Equal(t, "1", msg.ReqData)
	})
	t.Run("encode marshal append empty buf", func(t *testing.T) {
		data, err := EncodingProto.(protoEncoding).MarshalAppend(nil, &testservice.StreamMessage{ReqData: "1"})
		require.NoError(t, err)
		msg := &testservice.StreamMessage{}
		require.NoError(t, EncodingProto.Unmarshal(data, msg))
		assert.Equal(t, "1", msg.ReqData)
	})
	t.Run("encode marshal append non-empty buf", func(t *testing.T) {
		buf := make([]byte, 150)
		_, err := rand.Read(buf)
		require.NoError(t, err)
		data, err := EncodingProto.(protoEncoding).MarshalAppend(buf, &testservice.StreamMessage{ReqData: "1"})
		require.NoError(t, err)
		msg := &testservice.StreamMessage{}
		require.NoError(t, EncodingProto.Unmarshal(data[150:], msg))
		require.Equal(t, buf, data[:150])
		assert.Equal(t, "1", msg.ReqData)
	})
}
