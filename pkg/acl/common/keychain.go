package common

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

type Keychain struct {
	decoder keys.Decoder
	keys    map[string]signingkey.PubKey
}

func NewKeychain() *Keychain {
	return &Keychain{
		decoder: signingkey.NewEDPubKeyDecoder(),
		keys:    make(map[string]signingkey.PubKey),
	}
}

func (k *Keychain) GetOrAdd(identity string) (signingkey.PubKey, error) {
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
