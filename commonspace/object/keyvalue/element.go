package keyvalue

import (
	"encoding/hex"

	"github.com/anyproto/any-store/anyenc"
	"github.com/zeebo/blake3"
)

type KeyValue struct {
	Key            string
	Value          []byte
	TimestampMilli int
}

func (kv KeyValue) AnyEnc(a *anyenc.Arena) *anyenc.Value {
	obj := a.NewObject()
	obj.Set("id", a.NewString(kv.Key))
	if len(kv.Value) == 0 {
		obj.Set("d", a.NewTrue())
	} else {
		obj.Set("v", a.NewBinary(kv.Value))
		hash := blake3.Sum256(kv.Value)
		obj.Set("h", a.NewString(hex.EncodeToString(hash[:])))
	}
	obj.Set("t", a.NewNumberInt(kv.TimestampMilli))
	return obj
}
