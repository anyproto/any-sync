package secureservice

import (
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/util/crypto"
	"go.uber.org/zap"
)

func newNoVerifyChecker(protoVersion uint32) handshake.CredentialChecker {
	return &noVerifyChecker{
		cred: &handshakeproto.Credentials{Type: handshakeproto.CredentialsType_SkipVerify, Version: protoVersion},
	}
}

type noVerifyChecker struct {
	cred *handshakeproto.Credentials
}

func (n noVerifyChecker) MakeCredentials(remotePeerId string) *handshakeproto.Credentials {
	return n.cred
}

func (n noVerifyChecker) CheckCredential(remotePeerId string, cred *handshakeproto.Credentials) (identity []byte, err error) {
	if cred.Version != n.cred.Version {
		return nil, handshake.ErrIncompatibleVersion
	}
	return nil, nil
}

func newPeerSignVerifier(protoVersion uint32, account *accountdata.AccountKeys) handshake.CredentialChecker {
	return &peerSignVerifier{
		protoVersion: protoVersion,
		account:      account,
	}
}

type peerSignVerifier struct {
	protoVersion uint32
	account      *accountdata.AccountKeys
}

func (p *peerSignVerifier) MakeCredentials(remotePeerId string) *handshakeproto.Credentials {
	sign, err := p.account.SignKey.Sign([]byte(p.account.PeerId + remotePeerId))
	if err != nil {
		log.Warn("can't sign identity credentials", zap.Error(err))
	}
	// this will actually be called only once
	marshalled, _ := p.account.SignKey.GetPublic().Marshall()
	msg := &handshakeproto.PayloadSignedPeerIds{
		Identity: marshalled,
		Sign:     sign,
	}
	payload, _ := msg.Marshal()
	return &handshakeproto.Credentials{
		Type:    handshakeproto.CredentialsType_SignedPeerIds,
		Payload: payload,
		Version: p.protoVersion,
	}
}

func (p *peerSignVerifier) CheckCredential(remotePeerId string, cred *handshakeproto.Credentials) (identity []byte, err error) {
	if cred.Version != p.protoVersion {
		return nil, handshake.ErrIncompatibleVersion
	}
	if cred.Type != handshakeproto.CredentialsType_SignedPeerIds {
		return nil, handshake.ErrSkipVerifyNotAllowed
	}
	var msg = &handshakeproto.PayloadSignedPeerIds{}
	if err = msg.Unmarshal(cred.Payload); err != nil {
		return nil, handshake.ErrUnexpectedPayload
	}
	pubKey, err := crypto.UnmarshalEd25519PublicKeyProto(msg.Identity)
	if err != nil {
		return nil, handshake.ErrInvalidCredentials
	}
	ok, err := pubKey.Verify([]byte((remotePeerId + p.account.PeerId)), msg.Sign)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, handshake.ErrInvalidCredentials
	}
	return msg.Identity, nil
}
