package accountdata

import (
	"crypto/rand"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/anytypeio/any-sync/util/peer"
)

type AccountKeys struct {
	PeerKey crypto.PrivKey
	SignKey crypto.PrivKey
	PeerId  string
}

func New(peerKey crypto.PrivKey, signKey crypto.PrivKey) *AccountKeys {
	peerId, _ := peer.IdFromSigningPubKey(peerKey.GetPublic())
	return &AccountKeys{
		PeerKey: peerKey,
		SignKey: signKey,
		PeerId:  peerId.String(),
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
	peerId, err := peer.IdFromSigningPubKey(peerKey.GetPublic())
	if err != nil {
		return nil, err
	}
	return &AccountKeys{
		PeerKey: peerKey,
		SignKey: signKey,
		PeerId:  peerId.String(),
	}, nil
}
