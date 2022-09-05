package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/gogo/protobuf/proto"
)

type SignableChangeContent struct {
	Proto      proto.Marshaler
	Key        signingkey.PrivKey
	Identity   string
	IsSnapshot bool
}
