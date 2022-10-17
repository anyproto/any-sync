package acllistbuilder

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	encryptionkey2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	signingkey2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"hash/fnv"
	"strings"
)

type SymKey struct {
	Hash uint64
	Key  *symmetric.Key
}

type YAMLKeychain struct {
	SigningKeysByYAMLIdentity    map[string]signingkey2.PrivKey
	SigningKeysByRealIdentity    map[string]signingkey2.PrivKey
	EncryptionKeysByYAMLIdentity map[string]encryptionkey2.PrivKey
	ReadKeysByYAMLIdentity       map[string]*SymKey
	ReadKeysByHash               map[uint64]*SymKey
	GeneratedIdentities          map[string]string
	DerivedIdentity              string
}

func NewKeychain() *YAMLKeychain {
	return &YAMLKeychain{
		SigningKeysByYAMLIdentity:    map[string]signingkey2.PrivKey{},
		SigningKeysByRealIdentity:    map[string]signingkey2.PrivKey{},
		EncryptionKeysByYAMLIdentity: map[string]encryptionkey2.PrivKey{},
		GeneratedIdentities:          map[string]string{},
		ReadKeysByYAMLIdentity:       map[string]*SymKey{},
		ReadKeysByHash:               map[uint64]*SymKey{},
	}
}

func (k *YAMLKeychain) ParseKeys(keys *Keys) {
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

func (k *YAMLKeychain) AddEncryptionKey(key *Key) {
	if _, exists := k.EncryptionKeysByYAMLIdentity[key.Name]; exists {
		return
	}
	var (
		newPrivKey encryptionkey2.PrivKey
		err        error
	)
	if key.Value == "generated" {
		newPrivKey, _, err = encryptionkey2.GenerateRandomRSAKeyPair(2048)
		if err != nil {
			panic(err)
		}
	} else {
		newPrivKey, err = keys.DecodeKeyFromString(key.Value, encryptionkey2.NewEncryptionRsaPrivKeyFromBytes, nil)
		if err != nil {
			panic(err)
		}
	}
	k.EncryptionKeysByYAMLIdentity[key.Name] = newPrivKey
}

func (k *YAMLKeychain) AddSigningKey(key *Key) {
	if _, exists := k.SigningKeysByYAMLIdentity[key.Name]; exists {
		return
	}
	var (
		newPrivKey signingkey2.PrivKey
		pubKey     signingkey2.PubKey
		err        error
	)
	if key.Value == "generated" {
		newPrivKey, pubKey, err = signingkey2.GenerateRandomEd25519KeyPair()
		if err != nil {
			panic(err)
		}
	} else {
		newPrivKey, err = keys.DecodeKeyFromString(key.Value, signingkey2.NewSigningEd25519PrivKeyFromBytes, nil)
		if err != nil {
			panic(err)
		}
		pubKey = newPrivKey.GetPublic()
	}

	k.SigningKeysByYAMLIdentity[key.Name] = newPrivKey
	rawPubKey, err := pubKey.Raw()
	if err != nil {
		panic(err)
	}
	encoded := string(rawPubKey)

	k.SigningKeysByRealIdentity[encoded] = newPrivKey
	k.GeneratedIdentities[key.Name] = encoded
}

func (k *YAMLKeychain) AddReadKey(key *Key) {
	if _, exists := k.ReadKeysByYAMLIdentity[key.Name]; exists {
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
		signKey, _ := k.SigningKeysByYAMLIdentity[k.DerivedIdentity].Raw()
		encKey, _ := k.EncryptionKeysByYAMLIdentity[k.DerivedIdentity].Raw()
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

	k.ReadKeysByYAMLIdentity[key.Name] = &SymKey{
		Hash: hasher.Sum64(),
		Key:  rkey,
	}
	k.ReadKeysByHash[hasher.Sum64()] = &SymKey{
		Hash: hasher.Sum64(),
		Key:  rkey,
	}
}

func (k *YAMLKeychain) AddKey(key *Key) {
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

func (k *YAMLKeychain) GetKey(key string) interface{} {
	parts := strings.Split(key, ".")
	if len(parts) != 3 {
		panic("cannot parse a key")
	}
	name := parts[2]

	switch parts[1] {
	case "Sign":
		if key, exists := k.SigningKeysByYAMLIdentity[name]; exists {
			return key
		}
	case "Enc":
		if key, exists := k.EncryptionKeysByYAMLIdentity[name]; exists {
			return key
		}
	case "Read":
		if key, exists := k.ReadKeysByYAMLIdentity[name]; exists {
			return key
		}
	default:
		panic("incorrect format")
	}
	return nil
}

func (k *YAMLKeychain) GetIdentity(name string) string {
	return k.GeneratedIdentities[name]
}
