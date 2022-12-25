package objecttree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
)

type SignableChangeContent struct {
	Data        []byte
	Key         signingkey.PrivKey
	Identity    []byte
	IsSnapshot  bool
	IsEncrypted bool
}
