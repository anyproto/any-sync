package accountdata

import (
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
)

type AccountData struct { // TODO: create a convenient constructor for this
	Identity []byte // public key
	PeerKey  signingkey.PrivKey
	SignKey  signingkey.PrivKey
	EncKey   encryptionkey.PrivKey
	PeerId   string
}
