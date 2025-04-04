package coordinatorproto

import (
	"bytes"
	"time"

	"github.com/anyproto/any-sync/util/crypto"
)

func PrepareDeleteConfirmation(privKey crypto.PrivKey, spaceId, peerId, networkId string) (signed *DeletionConfirmPayloadWithSignature, err error) {
	marshalledIdentity, err := privKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	deleteConfirm := &DeletionConfirmPayload{
		SpaceId:         spaceId,
		PeerId:          peerId,
		AccountIdentity: marshalledIdentity,
		NetworkId:       networkId,
		Timestamp:       time.Now().Unix(),
	}
	marshalledDeleteConfirm, err := deleteConfirm.MarshalVT()
	if err != nil {
		return
	}
	signature, err := privKey.Sign(marshalledDeleteConfirm)
	if err != nil {
		return
	}
	signed = &DeletionConfirmPayloadWithSignature{
		DeletionPayload: marshalledDeleteConfirm,
		Signature:       signature,
	}
	return
}

func PrepareAccountDeleteConfirmation(privKey crypto.PrivKey, peerId, networkId string) (signed *DeletionConfirmPayloadWithSignature, err error) {
	marshalledIdentity, err := privKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	deleteConfirm := &AccountDeletionConfirmPayload{
		PeerId:          peerId,
		AccountIdentity: marshalledIdentity,
		NetworkId:       networkId,
		Timestamp:       time.Now().Unix(),
	}
	marshalledDeleteConfirm, err := deleteConfirm.MarshalVT()
	if err != nil {
		return
	}
	signature, err := privKey.Sign(marshalledDeleteConfirm)
	if err != nil {
		return
	}
	signed = &DeletionConfirmPayloadWithSignature{
		DeletionPayload: marshalledDeleteConfirm,
		Signature:       signature,
	}
	return
}

func ValidateAccountDeleteConfirmation(pubKey crypto.PubKey, spaceId, networkId string, deleteConfirm *DeletionConfirmPayloadWithSignature) (err error) {
	res, err := pubKey.Verify(deleteConfirm.GetDeletionPayload(), deleteConfirm.GetSignature())
	if err != nil {
		return
	}
	if !res {
		return errSignatureIncorrect
	}
	payload := &AccountDeletionConfirmPayload{}
	err = payload.UnmarshalVT(deleteConfirm.GetDeletionPayload())
	if err != nil {
		return
	}
	if payload.NetworkId != networkId {
		return errNetworkIsIncorrect
	}
	accountRaw, err := crypto.UnmarshalEd25519PublicKeyProto(payload.AccountIdentity)
	if err != nil {
		return
	}
	if !bytes.Equal(pubKey.Storage(), accountRaw.Storage()) {
		return errAccountIncorrect
	}
	return
}

func ValidateDeleteConfirmation(pubKey crypto.PubKey, spaceId, networkId string, deleteConfirm *DeletionConfirmPayloadWithSignature) (err error) {
	res, err := pubKey.Verify(deleteConfirm.GetDeletionPayload(), deleteConfirm.GetSignature())
	if err != nil {
		return
	}
	if !res {
		return errSignatureIncorrect
	}
	payload := &DeletionConfirmPayload{}
	err = payload.UnmarshalVT(deleteConfirm.GetDeletionPayload())
	if err != nil {
		return
	}
	if payload.SpaceId != spaceId {
		return errSpaceIdIncorrect
	}
	if payload.NetworkId != networkId {
		return errNetworkIsIncorrect
	}
	accountRaw, err := crypto.UnmarshalEd25519PublicKeyProto(payload.AccountIdentity)
	if err != nil {
		return
	}
	if !bytes.Equal(pubKey.Storage(), accountRaw.Storage()) {
		return errAccountIncorrect
	}
	return
}
