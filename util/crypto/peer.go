package crypto

import (
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func IdFromSigningPubKey(pubKey PubKey) (peer.ID, error) {
	rawSigning, err := pubKey.Raw()
	if err != nil {
		return "", err
	}
	libp2pKey, err := crypto.UnmarshalEd25519PublicKey(rawSigning)
	if err != nil {
		return "", err
	}
	return peer.IDFromPublicKey(libp2pKey)
}
