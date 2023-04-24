package commonspace

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/crypto"
	"golang.org/x/exp/slices"
)

func CheckResponsible(ctx context.Context, confService nodeconf.Service, spaceId string) (err error) {
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return
	}
	if isClient(confService, peerId) && !confService.IsResponsible(spaceId) {
		return spacesyncproto.ErrPeerIsNotResponsible
	}
	return
}

func isClient(confService nodeconf.Service, peerId string) bool {
	return len(confService.NodeTypes(peerId)) == 0
}

func checkCoordinator(confService nodeconf.Service, identity []byte, payload, signature []byte) (err error) {
	controlKey, err := crypto.UnmarshalEd25519PublicKey(identity)
	if err != nil {
		return
	}
	nodeTypes := confService.NodeTypes(controlKey.PeerId())
	if len(nodeTypes) == 0 || !slices.Contains(nodeTypes, nodeconf.NodeTypeCoordinator) {
		return errNoSuchCoordinatorNode
	}
	res, err := controlKey.Verify(payload, signature)
	if err != nil {
		return
	}
	if !res {
		return errReceiptSignatureIncorrect
	}
	return
}
