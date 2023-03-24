package encryptionkey

import (
	"github.com/anytypeio/any-sync/util/crypto"
)

type PrivKey interface {
	crypto.Key

	Decrypt([]byte) ([]byte, error)
	GetPublic() PubKey
}

type PubKey interface {
	crypto.Key

	Encrypt(data []byte) ([]byte, error)
}
