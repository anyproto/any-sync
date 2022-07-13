package account

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

type AccountData struct {
	Identity string
	SignKey  keys.SigningPrivKey
	EncKey   keys.EncryptionPrivKey
}
