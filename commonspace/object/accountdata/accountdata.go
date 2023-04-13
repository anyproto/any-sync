package accountdata

import (
	"crypto/rand"
	"github.com/anytypeio/any-sync/util/crypto"
)

type AccountKeys struct {
	PeerKey crypto.PrivKey
	SignKey crypto.PrivKey
	PeerId  string
}

func New(peerKey, signKey crypto.PrivKey) *AccountKeys {
	return &AccountKeys{
		PeerKey: peerKey,
		SignKey: signKey,
		PeerId:  peerKey.GetPublic().PeerId(),
	}
}

func NewRandom() (*AccountKeys, error) {
	peerKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	signKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &AccountKeys{
		PeerKey: peerKey,
		SignKey: signKey,
		PeerId:  peerKey.GetPublic().PeerId(),
	}, nil
}
