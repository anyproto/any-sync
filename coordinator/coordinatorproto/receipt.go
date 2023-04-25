package coordinatorproto

import (
	"bytes"
	"errors"
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
	errReceiptAccountIncorrect   = errors.New("receipt account is incorrect")
	errReceiptExpired            = errors.New("receipt is expired")
)

func PrepareSpaceReceipt(spaceId, peerId string, validPeriod time.Duration, accountPubKey crypto.PubKey, nodeKey crypto.PrivKey) (signedReceipt *SpaceReceiptWithSignature, err error) {
	marshalledAccount, err := accountPubKey.Marshall()
	if err != nil {
		return
	}
	marshalledNode, err := nodeKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	receipt := &SpaceReceipt{
		SpaceId:             spaceId,
		PeerId:              peerId,
		AccountIdentity:     marshalledAccount,
		ControlNodeIdentity: marshalledNode,
		ValidUntil:          uint64(time.Now().Add(validPeriod).Unix()),
	}
	receiptData, err := receipt.Marshal()
	if err != nil {
		return
	}
	sign, err := nodeKey.Sign(receiptData)
	if err != nil {
		return
	}
	return &SpaceReceiptWithSignature{
		SpaceReceiptPayload: receiptData,
		Signature:           sign,
	}, nil
}

func CheckReceipt(peerId, spaceId string, accountIdentity []byte, coordinators []string, receipt *SpaceReceiptWithSignature) (err error) {
	payload := &SpaceReceipt{}
	err = proto.Unmarshal(receipt.GetSpaceReceiptPayload(), payload)
	if err != nil {
		return
	}
	if payload.SpaceId != spaceId {
		return errReceiptSpaceIdIncorrect
	}
	if payload.PeerId != peerId {
		return errReceiptPeerIdIncorrect
	}
	protoRaw, err := crypto.UnmarshalEd25519PublicKeyProto(payload.AccountIdentity)
	if err != nil {
		return
	}
	accountRaw, err := crypto.UnmarshalEd25519PublicKeyProto(accountIdentity)
	if err != nil {
		return
	}
	if !bytes.Equal(protoRaw.Storage(), accountRaw.Storage()) {
		return errReceiptAccountIncorrect
	}
	err = checkCoordinator(
		coordinators,
		payload.ControlNodeIdentity,
		receipt.GetSpaceReceiptPayload(),
		receipt.GetSignature())
	if err != nil {
		return
	}
	if payload.GetValidUntil() <= uint64(time.Now().Unix()) {
		return errReceiptExpired
	}
	return
}

func checkCoordinator(coordinators []string, identity []byte, payload, signature []byte) (err error) {
	coordinatorKey, err := crypto.UnmarshalEd25519PublicKeyProto(identity)
	if err != nil {
		return
	}
	if !slices.Contains(coordinators, coordinatorKey.PeerId()) {
		return errNoSuchCoordinatorNode
	}
	res, err := coordinatorKey.Verify(payload, signature)
	if err != nil {
		return
	}
	if !res {
		return errReceiptSignatureIncorrect
	}
	return
}
