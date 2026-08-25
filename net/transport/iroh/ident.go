package iroh

import (
	"fmt"

	libp2ppeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/tmc/go-iroh/key"

	"github.com/anyproto/any-sync/util/crypto"
)

// The iroh endpoint id is the device's ed25519 peer key, so the any-sync
// peer id (a libp2p identity multihash of that key) and the endpoint id
// convert both ways without extra state.

// PeerIdFromEndpointId returns the any-sync peer id of an iroh endpoint.
func PeerIdFromEndpointId(id key.EndpointID) (string, error) {
	b := id.Bytes()
	pub, err := crypto.NewSigningEd25519PubKeyFromBytes(b[:])
	if err != nil {
		return "", err
	}
	peerId, err := crypto.IdFromSigningPubKey(pub)
	if err != nil {
		return "", err
	}
	return peerId.String(), nil
}

// EndpointIdFromPeerId returns the iroh endpoint id of an any-sync peer.
func EndpointIdFromPeerId(peerId string) (key.EndpointID, error) {
	id, err := libp2ppeer.Decode(peerId)
	if err != nil {
		return key.EndpointID{}, err
	}
	pub, err := id.ExtractPublicKey()
	if err != nil {
		return key.EndpointID{}, fmt.Errorf("peer id %s carries no public key: %w", peerId, err)
	}
	raw, err := pub.Raw()
	if err != nil {
		return key.EndpointID{}, err
	}
	return key.EndpointIDFromSlice(raw)
}
