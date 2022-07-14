package keys

import "crypto/subtle"

type Key interface {
	Equals(Key) bool

	Raw() ([]byte, error)
}

type Decoder interface {
	DecodeFromBytes(bytes []byte) (Key, error)
	DecodeFromString(identity string) (Key, error)
	DecodeFromStringIntoBytes(identity string) ([]byte, error)
	EncodeToString(key Key) (string, error)
}

func KeyEquals(k1, k2 Key) bool {
	a, err := k1.Raw()
	if err != nil {
		return false
	}
	b, err := k2.Raw()
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}
