package account

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

type AccountData struct { // TODO: create a convenient constructor for this
	Identity string // TODO: this is essentially the same as sign key
	SignKey  keys.SigningPrivKey
	EncKey   keys.EncryptionPrivKey
	Decoder  keys.SigningPubKeyDecoder
}
