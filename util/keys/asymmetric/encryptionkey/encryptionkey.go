package encryptionkey

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

type EncryptionPrivKey interface {
	keys.Key

	Decrypt([]byte) ([]byte, error)
	GetPublic() EncryptionPubKey
}

type EncryptionPubKey interface {
	keys.Key

	Encrypt(data []byte) ([]byte, error)
}
