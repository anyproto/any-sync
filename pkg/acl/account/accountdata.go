package account

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

type AccountData struct { // TODO: create a convenient constructor for this
	// Identity is non utf8 encoded, but we use this type, to eliminate copying between []byte to string conversions
	Identity string
	SignKey  signingkey.PrivKey
	EncKey   encryptionkey.PrivKey
	Decoder  keys.Decoder
}
