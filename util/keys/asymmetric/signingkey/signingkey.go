package signingkey

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

type SigningPrivKey interface {
	keys.Key

	Sign([]byte) ([]byte, error)

	GetPublic() SigningPubKey
}

type SigningPubKey interface {
	keys.Key

	Verify(data []byte, sig []byte) (bool, error)
}

type SigningPubKeyDecoder interface {
	DecodeFromBytes(bytes []byte) (SigningPubKey, error)
	DecodeFromString(identity string) (SigningPubKey, error)
	DecodeFromStringIntoBytes(identity string) ([]byte, error)
	EncodeToString(pubkey SigningPubKey) (string, error)
}
