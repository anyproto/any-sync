package encryptionkey

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
)

type PrivKey interface {
	keys.Key

	Decrypt([]byte) ([]byte, error)
	GetPublic() PubKey
}

type PubKey interface {
	keys.Key

	Encrypt(data []byte) ([]byte, error)
}
