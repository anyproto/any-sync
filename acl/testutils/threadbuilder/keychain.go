package threadbuilder

import (
	"hash/fnv"
	"strings"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

	"github.com/textileio/go-threads/crypto/symmetric"
)

type SymKey struct {
	Hash uint64
	Key  *symmetric.Key
}

type Keychain struct {
	SigningKeys           map[string]keys.SigningPrivKey
	SigningKeysByIdentity map[string]keys.SigningPrivKey
	EncryptionKeys        map[string]keys.EncryptionPrivKey
	ReadKeys              map[string]*SymKey
	ReadKeysByHash        map[uint64]*SymKey
	GeneratedIdentities   map[string]string
	coder                 *keys.Ed25519SigningPubKeyDecoder
}

func NewKeychain() *Keychain {
	return &Keychain{
		SigningKeys:           map[string]keys.SigningPrivKey{},
		SigningKeysByIdentity: map[string]keys.SigningPrivKey{},
		EncryptionKeys:        map[string]keys.EncryptionPrivKey{},
		GeneratedIdentities:   map[string]string{},
		ReadKeys:              map[string]*SymKey{},
		ReadKeysByHash:        map[uint64]*SymKey{},
		coder:                 keys.NewEd25519Decoder(),
	}
}

func (k *Keychain) ParseKeys(keys *Keys) {
	for _, encKey := range keys.Enc {
		k.AddEncryptionKey(encKey)
	}

	for _, signKey := range keys.Sign {
		k.AddSigningKey(signKey)
	}

	for _, readKey := range keys.Read {
		k.AddReadKey(readKey)
	}
}

func (k *Keychain) AddEncryptionKey(name string) {
	if _, exists := k.EncryptionKeys[name]; exists {
		return
	}
	newPrivKey, _, err := keys.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		panic(err)
	}

	k.EncryptionKeys[name] = newPrivKey
}

func (k *Keychain) AddSigningKey(name string) {
	if _, exists := k.SigningKeys[name]; exists {
		return
	}
	newPrivKey, pubKey, err := keys.GenerateRandomEd25519KeyPair()
	if err != nil {
		panic(err)
	}

	k.SigningKeys[name] = newPrivKey
	res, err := k.coder.EncodeToString(pubKey)
	if err != nil {
		panic(err)
	}
	k.SigningKeysByIdentity[res] = newPrivKey
	k.GeneratedIdentities[name] = res
}

func (k *Keychain) AddReadKey(name string) {
	if _, exists := k.ReadKeys[name]; exists {
		return
	}
	key, _ := symmetric.NewRandom()

	hasher := fnv.New64()
	hasher.Write(key.Bytes())

	k.ReadKeys[name] = &SymKey{
		Hash: hasher.Sum64(),
		Key:  key,
	}
	k.ReadKeysByHash[hasher.Sum64()] = &SymKey{
		Hash: hasher.Sum64(),
		Key:  key,
	}
}

func (k *Keychain) AddKey(key string) {
	parts := strings.Split(key, ".")
	if len(parts) != 3 {
		panic("cannot parse a key")
	}
	name := parts[2]

	switch parts[1] {
	case "Sign":
		k.AddSigningKey(name)
	case "Enc":
		k.AddEncryptionKey(name)
	case "Read":
		k.AddReadKey(name)
	default:
		panic("incorrect format")
	}
}

func (k *Keychain) GetKey(key string) interface{} {
	parts := strings.Split(key, ".")
	if len(parts) != 3 {
		panic("cannot parse a key")
	}
	name := parts[2]

	switch parts[1] {
	case "Sign":
		if key, exists := k.SigningKeys[name]; exists {
			return key
		}
	case "Enc":
		if key, exists := k.EncryptionKeys[name]; exists {
			return key
		}
	case "Read":
		if key, exists := k.ReadKeys[name]; exists {
			return key
		}
	default:
		panic("incorrect format")
	}
	return nil
}

func (k *Keychain) GetIdentity(name string) string {
	return k.GeneratedIdentities[name]
}
