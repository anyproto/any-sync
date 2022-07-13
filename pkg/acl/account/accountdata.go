package account

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

type AccountData struct {
	Identity string // TODO: this is essentially the same as sign key
	SignKey  keys.SigningPrivKey
	EncKey   keys.EncryptionPrivKey
}
