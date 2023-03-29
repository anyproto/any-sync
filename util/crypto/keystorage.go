package crypto

import "bytes"

type KeyStorage interface {
	PubKeyFromProto(protoBytes []byte) (PubKey, error)
}

func NewKeyStorage() KeyStorage {
	return &keyStorage{}
}

type pubKeyEntry struct {
	protoKey []byte
	key      PubKey
}

type keyStorage struct {
	keys []pubKeyEntry
}

func (k *keyStorage) PubKeyFromProto(protoBytes []byte) (PubKey, error) {
	for _, k := range k.keys {
		// it is not guaranteed that proto will always marshal to the same bytes (but in our case it probably will)
		// but this shouldn't be the problem, because we will just create another copy
		if bytes.Equal(protoBytes, k.protoKey) {
			return k.key, nil
		}
	}
	key, err := UnmarshalEd25519PublicKeyProto(protoBytes)
	if err != nil {
		return nil, err
	}
	k.keys = append(k.keys, pubKeyEntry{
		protoKey: protoBytes,
		key:      key,
	})
	return key, nil
}
