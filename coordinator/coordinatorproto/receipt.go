package coordinatorproto

import (
	"bytes"
	"errors"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/strkey"
	"time"
)

var (
	errSignatureIncorrect = errors.New("receipt signature is incorrect")
	errNetworkIsIncorrect = errors.New("network is incorrect")
	errSpaceIdIncorrect   = errors.New("receipt space id is incorrect")
	errPeerIdIncorrect    = errors.New("receipt peer id is incorrect")
	errAccountIncorrect   = errors.New("receipt account is incorrect")
	errReceiptExpired     = errors.New("receipt is expired")
)

func PrepareSpaceReceipt(spaceId, peerId string, validPeriod time.Duration, accountPubKey crypto.PubKey, networkKey crypto.PrivKey) (signedReceipt *SpaceReceiptWithSignature, err error) {
	marshalledAccount, err := accountPubKey.Marshall()
	if err != nil {
		return
	}
	receipt := &SpaceReceipt{
		SpaceId:         spaceId,
		PeerId:          peerId,
		AccountIdentity: marshalledAccount,
		NetworkId:       networkKey.GetPublic().Network(),
		ValidUntil:      uint64(time.Now().Add(validPeriod).Unix()),
	}
	receiptData, err := receipt.MarshalVT()
	if err != nil {
		return
	}
	sign, err := networkKey.Sign(receiptData)
	if err != nil {
		return
	}
	return &SpaceReceiptWithSignature{
		SpaceReceiptPayload: receiptData,
		Signature:           sign,
	}, nil
}

func CheckReceipt(peerId, spaceId string, accountIdentity []byte, networkId string, receipt *SpaceReceiptWithSignature) (err error) {
	payload := &SpaceReceipt{}
	err = payload.UnmarshalVT(receipt.GetSpaceReceiptPayload())
	if err != nil {
		return
	}
	if payload.SpaceId != spaceId {
		return errSpaceIdIncorrect
	}
	if payload.PeerId != peerId {
		return errPeerIdIncorrect
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
		return errAccountIncorrect
	}
	err = checkNetwork(
		networkId,
		payload.NetworkId,
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

func checkNetwork(networkId, payloadNetworkId string, payload, signature []byte) (err error) {
	if networkId != payloadNetworkId {
		return errNetworkIsIncorrect
	}
	networkIdentity, err := strkey.Decode(strkey.NetworkAddressVersionByte, networkId)
	if err != nil {
		return
	}
	networkKey, err := crypto.UnmarshalEd25519PublicKey(networkIdentity)
	if err != nil {
		return
	}
	res, err := networkKey.Verify(payload, signature)
	if err != nil {
		return
	}
	if !res {
		return errSignatureIncorrect
	}
	return
}
