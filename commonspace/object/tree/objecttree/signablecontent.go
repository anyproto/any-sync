package objecttree

import (
	"github.com/anytypeio/any-sync/util/crypto"
)

type SignableChangeContent struct {
	Data        []byte
	Key         crypto.PrivKey
	IsSnapshot  bool
	IsEncrypted bool
}
