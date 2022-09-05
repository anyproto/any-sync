package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

type keychain struct {
	decoder keys.Decoder
	keys    map[string]signingkey.PubKey
}

func newKeychain() *keychain {
	return &keychain{
		decoder: signingkey.NewEDPubKeyDecoder(),
	}
}

func (k *keychain) getOrAdd(identity string) (signingkey.PubKey, error) {
	if key, exists := k.keys[identity]; exists {
		return key, nil
	}
	res, err := k.decoder.DecodeFromString(identity)
	if err != nil {
		return nil, err
	}

	k.keys[identity] = res.(signingkey.PubKey)
	return res.(signingkey.PubKey), nil
}
