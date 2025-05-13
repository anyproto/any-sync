package innerstorage

import (
	"errors"

	"github.com/anyproto/any-store/anyenc"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/util/crypto"
)

var ErrInvalidSignature = errors.New("invalid signature")

type KeyValue struct {
	KeyPeerId      string
	ReadKeyId      string
	Key            string
	Value          Value
	TimestampMilli int
	Identity       string
	PeerId         string
	AclId          string
}

type Value struct {
	Value             []byte
	PeerSignature     []byte
	IdentitySignature []byte
}

func KeyValueFromProto(proto *spacesyncproto.StoreKeyValue, verify bool) (kv KeyValue, err error) {
	kv.KeyPeerId = proto.KeyPeerId
	kv.Value.Value = proto.Value
	kv.Value.PeerSignature = proto.PeerSignature
	kv.Value.IdentitySignature = proto.IdentitySignature
	innerValue := &spacesyncproto.StoreKeyInner{}
	if err = innerValue.UnmarshalVT(proto.Value); err != nil {
		return kv, err
	}
	kv.TimestampMilli = int(innerValue.TimestampMicro)
	identity, err := crypto.UnmarshalEd25519PublicKeyProto(innerValue.Identity)
	if err != nil {
		return kv, err
	}
	peerId, err := crypto.UnmarshalEd25519PublicKeyProto(innerValue.Peer)
	if err != nil {
		return kv, err
	}
	kv.Identity = identity.Account()
	kv.PeerId = peerId.PeerId()
	kv.Key = innerValue.Key
	kv.AclId = innerValue.AclHeadId
	// TODO: check that key-peerId is equal to key+peerId?
	if verify {
		if verify, _ = identity.Verify(proto.Value, proto.IdentitySignature); !verify {
			return kv, ErrInvalidSignature
		}
		if verify, _ = peerId.Verify(proto.Value, proto.PeerSignature); !verify {
			return kv, ErrInvalidSignature
		}
	}
	return kv, nil
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
	obj.Set("id", a.NewString(kv.KeyPeerId))
	obj.Set("k", a.NewString(kv.Key))
	obj.Set("r", a.NewString(kv.ReadKeyId))
	obj.Set("v", kv.Value.AnyEnc(a))
	obj.Set("t", a.NewNumberInt(kv.TimestampMilli))
	obj.Set("i", a.NewString(kv.Identity))
	obj.Set("p", a.NewString(kv.PeerId))
	return obj
}

func (kv KeyValue) Proto() *spacesyncproto.StoreKeyValue {
	return &spacesyncproto.StoreKeyValue{
		KeyPeerId:         kv.KeyPeerId,
		Value:             kv.Value.Value,
		PeerSignature:     kv.Value.PeerSignature,
		IdentitySignature: kv.Value.IdentitySignature,
	}
}
