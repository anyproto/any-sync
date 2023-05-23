package streampool

import (
	"github.com/anyproto/any-sync/net/streampool/testservice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProtoEncoding(t *testing.T) {
	t.Run("not a proto err", func(t *testing.T) {
		_, err := EncodingProto.Marshal("string")
		assert.Error(t, err)
		err = EncodingProto.Unmarshal(nil, "sss")
		assert.Error(t, err)
	})
	t.Run("encode", func(t *testing.T) {
		data, err := EncodingProto.Marshal(&testservice.StreamMessage{ReqData: "1"})
		require.NoError(t, err)
		msg := &testservice.StreamMessage{}
		require.NoError(t, EncodingProto.Unmarshal(data, msg))
		assert.Equal(t, "1", msg.ReqData)
	})
}
