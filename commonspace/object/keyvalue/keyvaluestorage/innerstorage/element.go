package innerstorage

import (
	"errors"

	"github.com/anyproto/any-store/anyenc"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/util/crypto"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidKeyPeerId = errors.New("keyPeerId does not match key and peer")
	ErrInvalidWatermark = errors.New("invalid deletion watermark")
)

// KeyPeerId is the row id of key written by peerId.
func KeyPeerId(key, peerId string) string {
	return key + "-" + peerId
}

type KeyValue struct {
	KeyPeerId string
	ReadKeyId string
	Key       string
	Value     Value
	// TimestampMicro is the writer-supplied LWW timestamp in microseconds
	// (StoreKeyInner.TimestampMicro). int64, not int: on 32-bit platforms an
	// int wraps and breaks both the LWW comparison and newest-first ordering.
	TimestampMicro int64
	Identity       string
	PeerId         string
	AclId          string
	// DeletePrefix marks the row as a deletion watermark (see
	// StoreKeyInner.deletePrefix). Key mirrors the prefix; the encrypted
	// payload stays empty.
	DeletePrefix string
	// IdentityPubKey is the parsed identity from the inner payload; set by
	// KeyValueFromProto for permission checks, never persisted.
	IdentityPubKey crypto.PubKey
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
	kv.TimestampMicro = innerValue.TimestampMicro
	identity, err := crypto.UnmarshalEd25519PublicKeyProto(innerValue.Identity)
	if err != nil {
		return kv, err
	}
	peerId, err := crypto.UnmarshalEd25519PublicKeyProto(innerValue.Peer)
	if err != nil {
		return kv, err
	}
	kv.Identity = identity.Account()
	kv.IdentityPubKey = identity
	kv.PeerId = peerId.PeerId()
	kv.Key = innerValue.Key
	kv.AclId = innerValue.AclHeadId
	kv.DeletePrefix = innerValue.GetDelete().GetPrefix()
	// The id must be derived from the signed payload: an unchecked KeyPeerId
	// would let any writer occupy (and overwrite) another peer's row.
	if kv.KeyPeerId != KeyPeerId(kv.Key, kv.PeerId) {
		return kv, ErrInvalidKeyPeerId
	}
	// A watermark's key mirrors its prefix, keeping it inside the key range
	// it governs (prefix iteration, id-derived invariants).
	if kv.DeletePrefix != "" && kv.Key != kv.DeletePrefix {
		return kv, ErrInvalidWatermark
	}
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
	// anyenc numbers are float64: exact for µs timestamps (< 2^53), and int-free
	// so the value survives 32-bit platforms.
	obj.Set("t", a.NewNumberFloat64(float64(kv.TimestampMicro)))
	obj.Set("i", a.NewString(kv.Identity))
	obj.Set("p", a.NewString(kv.PeerId))
	if kv.DeletePrefix != "" {
		obj.Set("dp", a.NewString(kv.DeletePrefix))
	}
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
