package coordinatorclient

import (
	"bytes"
	"context"
	"errors"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/coordinator/coordinatorproto"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/exp/slices"
	"time"
)

var (
	errReceiptSignatureIncorrect = errors.New("receipt signature is incorrect")
	errNoSuchCoordinatorNode     = errors.New("no such control node")
	errReceiptSpaceIdIncorrect   = errors.New("receipt space id is incorrect")
	errReceiptPeerIdIncorrect    = errors.New("receipt peer id is incorrect")
	errReceiptAccountIncorrect   = errors.New("receipt account id is incorrect")
	errReceiptExpired            = errors.New("receipt is expired")
)

func CheckReceipt(ctx context.Context, confService nodeconf.Service, request *spacesyncproto.SpacePushRequest) (err error) {
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return
	}
	accountIdentity, err := peer.CtxIdentity(ctx)
	if err != nil {
		return
	}
	credential := &coordinatorproto.SpaceReceiptWithSignature{}
	err = proto.Unmarshal(request.GetCredential(), credential)
	if err != nil {
		return
	}
	payload := &coordinatorproto.SpaceReceipt{}
	err = proto.Unmarshal(credential.GetSpaceReceiptPayload(), payload)
	if err != nil {
		return
	}
	if payload.GetSpaceId() != request.GetPayload().GetSpaceHeader().GetId() {
		return errReceiptSpaceIdIncorrect
	}
	if payload.GetPeerId() != peerId {
		return errReceiptPeerIdIncorrect
	}
	if !bytes.Equal(payload.GetAccountIdentity(), accountIdentity) {
		return errReceiptAccountIncorrect
	}
	err = checkCoordinator(
		confService,
		payload.GetControlNodeIdentity(),
		credential.GetSpaceReceiptPayload(),
		credential.GetSignature())
	if err != nil {
		return
	}
	if payload.GetValidUntil() < uint64(time.Now().Unix()) {
		return errReceiptExpired
	}
	return
}

func checkCoordinator(confService nodeconf.Service, identity []byte, payload, signature []byte) (err error) {
	cooordinatorKey, err := crypto.UnmarshalEd25519PublicKey(identity)
	if err != nil {
		return
	}
	nodeTypes := confService.NodeTypes(cooordinatorKey.PeerId())
	if len(nodeTypes) == 0 || !slices.Contains(nodeTypes, nodeconf.NodeTypeCoordinator) {
		return errNoSuchCoordinatorNode
	}
	res, err := cooordinatorKey.Verify(payload, signature)
	if err != nil {
		return
	}
	if !res {
		return errReceiptSignatureIncorrect
	}
	return
}
