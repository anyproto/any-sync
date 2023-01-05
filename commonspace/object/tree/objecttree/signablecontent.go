package objecttree

import (
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
)

type SignableChangeContent struct {
	Data        []byte
	Key         signingkey.PrivKey
	Identity    []byte
	IsSnapshot  bool
	IsEncrypted bool
}
