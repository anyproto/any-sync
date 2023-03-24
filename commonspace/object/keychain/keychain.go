package keychain

import (
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
)

type Keychain struct {
	keys map[string]signingkey.PubKey
}

func NewKeychain() *Keychain {
	return &Keychain{
		keys: make(map[string]signingkey.PubKey),
	}
}

func (k *Keychain) GetOrAdd(identity string) (signingkey.PubKey, error) {
	if key, exists := k.keys[identity]; exists {
		return key, nil
	}
	res, err := crypto.NewSigningEd25519PubKeyFromBytes([]byte(identity))
	if err != nil {
		return nil, err
	}

	k.keys[identity] = res.(signingkey.PubKey)
	return res.(signingkey.PubKey), nil
}
