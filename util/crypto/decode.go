package crypto

import (
	"encoding/base64"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/anyproto/any-sync/util/strkey"
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

func DecodeAccountAddress(address string) (PubKey, error) {
	pubKeyRaw, err := strkey.Decode(strkey.AccountAddressVersionByte, address)
	if err != nil {
		return nil, err
	}
	return UnmarshalEd25519PublicKey(pubKeyRaw)
}

func DecodePeerId(peerId string) (PubKey, error) {
	decoded, err := peer.Decode(peerId)
	if err != nil {
		return nil, err
	}

	pk, err := decoded.ExtractPublicKey()
	if err != nil {
		return nil, err
	}
	raw, err := pk.Raw()
	if err != nil {
		return nil, err
	}
	return UnmarshalEd25519PublicKey(raw)
}

func DecodeNetworkId(networkId string) (PubKey, error) {
	pubKeyRaw, err := strkey.Decode(strkey.NetworkAddressVersionByte, networkId)
	if err != nil {
		return nil, err
	}
	return UnmarshalEd25519PublicKey(pubKeyRaw)
}
