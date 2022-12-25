package acllistbuilder

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"hash/fnv"
	"strings"
)

type SymKey struct {
	Hash uint64
	Key  *symmetric.Key
}

type YAMLKeychain struct {
	SigningKeysByYAMLName     map[string]signingkey.PrivKey
	SigningKeysByRealIdentity map[string]signingkey.PrivKey
	EncryptionKeysByYAMLName  map[string]encryptionkey.PrivKey
	ReadKeysByYAMLName        map[string]*SymKey
	ReadKeysByHash            map[uint64]*SymKey
	GeneratedIdentities       map[string]string
}

func NewKeychain() *YAMLKeychain {
	return &YAMLKeychain{
		SigningKeysByYAMLName:     map[string]signingkey.PrivKey{},
		SigningKeysByRealIdentity: map[string]signingkey.PrivKey{},
		EncryptionKeysByYAMLName:  map[string]encryptionkey.PrivKey{},
		GeneratedIdentities:       map[string]string{},
		ReadKeysByYAMLName:        map[string]*SymKey{},
		ReadKeysByHash:            map[uint64]*SymKey{},
	}
}

func (k *YAMLKeychain) ParseKeys(keys *Keys) {
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
	if _, exists := k.EncryptionKeysByYAMLName[key.Name]; exists {
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
		newPrivKey, err = keys.DecodeKeyFromString(key.Value, encryptionkey.NewEncryptionRsaPrivKeyFromBytes, nil)
		if err != nil {
			panic(err)
		}
	}
	k.EncryptionKeysByYAMLName[key.Name] = newPrivKey
}

func (k *YAMLKeychain) AddSigningKey(key *Key) {
	if _, exists := k.SigningKeysByYAMLName[key.Name]; exists {
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
		newPrivKey, err = keys.DecodeKeyFromString(key.Value, signingkey.NewSigningEd25519PrivKeyFromBytes, nil)
		if err != nil {
			panic(err)
		}
		pubKey = newPrivKey.GetPublic()
	}

	k.SigningKeysByYAMLName[key.Name] = newPrivKey
	rawPubKey, err := pubKey.Raw()
	if err != nil {
		panic(err)
	}
	encoded := string(rawPubKey)

	k.SigningKeysByRealIdentity[encoded] = newPrivKey
	k.GeneratedIdentities[key.Name] = encoded
}

func (k *YAMLKeychain) AddReadKey(key *Key) {
	if _, exists := k.ReadKeysByYAMLName[key.Name]; exists {
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
		signKey, _ := k.SigningKeysByYAMLName[key.Name].Raw()
		encKey, _ := k.EncryptionKeysByYAMLName[key.Name].Raw()
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

	k.ReadKeysByYAMLName[key.Name] = &SymKey{
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
		if key, exists := k.SigningKeysByYAMLName[name]; exists {
			return key
		}
	case "Enc":
		if key, exists := k.EncryptionKeysByYAMLName[name]; exists {
			return key
		}
	case "Read":
		if key, exists := k.ReadKeysByYAMLName[name]; exists {
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
