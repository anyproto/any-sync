package keys

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/strkey"

type keyDecoder struct {
	create func([]byte) (Key, error)
}

func NewKeyDecoder(create func(bytes []byte) (Key, error)) Decoder {
	return &keyDecoder{
		create: create,
	}
}

func (e *keyDecoder) DecodeFromBytes(bytes []byte) (Key, error) {
	return e.create(bytes)
}

func (e *keyDecoder) DecodeFromString(identity string) (Key, error) {
	pubKeyRaw, err := strkey.Decode(0x5b, identity)
	if err != nil {
		return nil, err
	}

	return e.DecodeFromBytes(pubKeyRaw)
}

func (e *keyDecoder) DecodeFromStringIntoBytes(identity string) ([]byte, error) {
	return strkey.Decode(0x5b, identity)
}

func (e *keyDecoder) EncodeToString(key Key) (string, error) {
	raw, err := key.Raw()
	if err != nil {
		return "", err
	}
	return strkey.Encode(0x5b, raw)
}
