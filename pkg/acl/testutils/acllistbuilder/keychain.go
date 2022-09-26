package acllistbuilder

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"hash/fnv"
	"strings"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
)

type SymKey struct {
	Hash uint64
	Key  *symmetric.Key
}

type Keychain struct {
	SigningKeys           map[string]signingkey.PrivKey
	SigningKeysByIdentity map[string]signingkey.PrivKey
	EncryptionKeys        map[string]encryptionkey.PrivKey
	ReadKeys              map[string]*SymKey
	ReadKeysByHash        map[uint64]*SymKey
	GeneratedIdentities   map[string]string
	DerivedIdentity       string
	coder                 signingkey.PubKeyDecoder
}

func NewKeychain() *Keychain {
	return &Keychain{
		SigningKeys:           map[string]signingkey.PrivKey{},
		SigningKeysByIdentity: map[string]signingkey.PrivKey{},
		EncryptionKeys:        map[string]encryptionkey.PrivKey{},
		GeneratedIdentities:   map[string]string{},
		ReadKeys:              map[string]*SymKey{},
		ReadKeysByHash:        map[uint64]*SymKey{},
		coder:                 signingkey.NewEd25519PubKeyDecoder(),
	}
}

func (k *Keychain) ParseKeys(keys *Keys) {
	k.DerivedIdentity = keys.Derived
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

func (k *Keychain) AddEncryptionKey(key *Key) {
	if _, exists := k.EncryptionKeys[key.Name]; exists {
		return
	}
	var (
		newPrivKey encryptionkey.PrivKey
		err        error
	)
	if key.Value == "generated" {
		newPrivKey, _, err = encryptionkey.GenerateRandomRSAKeyPair(2048)
		if err != nil {
			panic(err)
		}
	} else {
		decoder := encryptionkey.NewRSAPrivKeyDecoder()
		privKey, err := decoder.DecodeFromString(key.Value)
		if err != nil {
			panic(err)
		}
		newPrivKey = privKey.(encryptionkey.PrivKey)
	}
	k.EncryptionKeys[key.Name] = newPrivKey
}

func (k *Keychain) AddSigningKey(key *Key) {
	if _, exists := k.SigningKeys[key.Name]; exists {
		return
	}
	var (
		newPrivKey signingkey.PrivKey
		pubKey     signingkey.PubKey
		err        error
	)
	if key.Value == "generated" {
		newPrivKey, pubKey, err = signingkey.GenerateRandomEd25519KeyPair()
		if err != nil {
			panic(err)
		}
	} else {
		decoder := signingkey.NewEDPrivKeyDecoder()
		privKey, err := decoder.DecodeFromString(key.Value)
		if err != nil {
			panic(err)
		}
		newPrivKey = privKey.(signingkey.PrivKey)
		pubKey = newPrivKey.GetPublic()
	}

	k.SigningKeys[key.Name] = newPrivKey
	rawPubKey, err := pubKey.Raw()
	if err != nil {
		panic(err)
	}
	encoded := string(rawPubKey)

	k.SigningKeysByIdentity[encoded] = newPrivKey
	k.GeneratedIdentities[key.Name] = encoded
}

func (k *Keychain) AddReadKey(key *Key) {
	if _, exists := k.ReadKeys[key.Name]; exists {
		return
	}

	var (
		rkey *symmetric.Key
		err  error
	)
	if key.Value == "generated" {
		rkey, err = symmetric.NewRandom()
		if err != nil {
			panic("should be able to generate symmetric key")
		}
	} else if key.Value == "derived" {
		signKey, _ := k.SigningKeys[k.DerivedIdentity].Raw()
		encKey, _ := k.EncryptionKeys[k.DerivedIdentity].Raw()
		rkey, err = aclrecordproto.ACLReadKeyDerive(signKey, encKey)
		if err != nil {
			panic("should be able to derive symmetric key")
		}
	} else {
		rkey, err = symmetric.FromString(key.Value)
		if err != nil {
			panic("should be able to parse symmetric key")
		}
	}

	hasher := fnv.New64()
	hasher.Write(rkey.Bytes())

	k.ReadKeys[key.Name] = &SymKey{
		Hash: hasher.Sum64(),
		Key:  rkey,
	}
	k.ReadKeysByHash[hasher.Sum64()] = &SymKey{
		Hash: hasher.Sum64(),
		Key:  rkey,
	}
}

func (k *Keychain) AddKey(key *Key) {
	parts := strings.Split(key.Name, ".")
	if len(parts) != 3 {
		panic("cannot parse a key")
	}

	switch parts[1] {
	case "Signature":
		k.AddSigningKey(key)
	case "Enc":
		k.AddEncryptionKey(key)
	case "Read":
		k.AddReadKey(key)
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
