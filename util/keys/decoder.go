package keys

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/strkey"

type keyDecoder[T Key] struct {
	create func([]byte) (T, error)
}

func NewKeyDecoder[T Key](create func(bytes []byte) (T, error)) Decoder {
	return &keyDecoder[T]{
		create: create,
	}
}

func (e *keyDecoder[T]) DecodeFromBytes(bytes []byte) (Key, error) {
	return e.create(bytes)
}

func (e *keyDecoder[T]) DecodeFromString(identity string) (Key, error) {
	pubKeyRaw, err := strkey.Decode(0x5b, identity)
	if err != nil {
		return nil, err
	}

	return e.DecodeFromBytes(pubKeyRaw)
}

func (e *keyDecoder[T]) DecodeFromStringIntoBytes(identity string) ([]byte, error) {
	return strkey.Decode(0x5b, identity)
}

func (e *keyDecoder[T]) EncodeToString(key Key) (string, error) {
	raw, err := key.Raw()
	if err != nil {
		return "", err
	}
	return strkey.Encode(0x5b, raw)
}
