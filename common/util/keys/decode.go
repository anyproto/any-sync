package keys

import (
	"encoding/base64"
)

func EncodeKeyToString[T Key](key T) (str string, err error) {
	raw, err := key.Raw()
	if err != nil {
		return
	}
	str = EncodeBytesToString(raw)
	return
}

func EncodeBytesToString(bytes []byte) string {
	return base64.StdEncoding.EncodeToString(bytes)
}

func DecodeKeyFromString[T Key](str string, construct func([]byte) (T, error), def T) (T, error) {
	dec, err := DecodeBytesFromString(str)
	if err != nil {
		return def, err
	}
	return construct(dec)
}

func DecodeBytesFromString(str string) (bytes []byte, err error) {
	return base64.StdEncoding.DecodeString(str)
}
