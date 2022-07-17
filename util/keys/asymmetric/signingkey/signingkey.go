package signingkey

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

type PrivKey interface {
	keys.Key

	Sign([]byte) ([]byte, error)

	GetPublic() PubKey
}

type PubKey interface {
	keys.Key

	Verify(data []byte, sig []byte) (bool, error)
}

type PubKeyDecoder interface {
	DecodeFromBytes(bytes []byte) (PubKey, error)
	DecodeFromString(identity string) (PubKey, error)
	DecodeFromStringIntoBytes(identity string) ([]byte, error)
	EncodeToString(pubkey PubKey) (string, error)
}
