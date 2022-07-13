package account

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

type AccountData struct { // TODO: create a convenient constructor for this
	Identity string // TODO: this is essentially the same as sign key
	SignKey  signingkey.SigningPrivKey
	EncKey   encryptionkey.EncryptionPrivKey
	Decoder  signingkey.SigningPubKeyDecoder
}
