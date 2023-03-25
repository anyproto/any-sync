package accountdata

import (
	"github.com/anytypeio/any-sync/util/crypto"
)

type AccountKeys struct {
	PeerKey crypto.PrivKey
	SignKey crypto.PrivKey
	PeerId  string
}
