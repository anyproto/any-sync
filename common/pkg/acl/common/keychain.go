package common

import (
	signingkey2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
)

type Keychain struct {
	keys map[string]signingkey2.PubKey
}

func NewKeychain() *Keychain {
	return &Keychain{
		keys: make(map[string]signingkey2.PubKey),
	}
}

func (k *Keychain) GetOrAdd(identity string) (signingkey2.PubKey, error) {
	if key, exists := k.keys[identity]; exists {
		return key, nil
	}
	res, err := signingkey2.NewSigningEd25519PubKeyFromBytes([]byte(identity))
	if err != nil {
		return nil, err
	}

	k.keys[identity] = res.(signingkey2.PubKey)
	return res.(signingkey2.PubKey), nil
}
