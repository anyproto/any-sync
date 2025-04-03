package innerstorage

import (
	"github.com/anyproto/any-store/anyenc"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type KeyValue struct {
	Key            string
	Value          Value
	TimestampMilli int
	Identity       string
	PeerId         string
}

type Value struct {
	Value             []byte
	PeerSignature     []byte
	IdentitySignature []byte
}

func (v Value) AnyEnc(a *anyenc.Arena) *anyenc.Value {
	obj := a.NewObject()
	obj.Set("v", a.NewBinary(v.Value))
	obj.Set("p", a.NewBinary(v.PeerSignature))
	obj.Set("i", a.NewBinary(v.IdentitySignature))
	return obj
}

func (kv KeyValue) AnyEnc(a *anyenc.Arena) *anyenc.Value {
	obj := a.NewObject()
	obj.Set("id", a.NewString(kv.Key))
	obj.Set("v", kv.Value.AnyEnc(a))
	obj.Set("t", a.NewNumberInt(kv.TimestampMilli))
	obj.Set("i", a.NewString(kv.Identity))
	obj.Set("p", a.NewString(kv.PeerId))
	return obj
}

func (kv KeyValue) Proto() *spacesyncproto.StoreKeyValue {
	return &spacesyncproto.StoreKeyValue{
		Key:               kv.Key,
		Value:             kv.Value.Value,
		PeerSignature:     kv.Value.PeerSignature,
		IdentitySignature: kv.Value.IdentitySignature,
	}
}
