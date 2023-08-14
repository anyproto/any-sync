package secureservice

import (
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/util/crypto"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

func newNoVerifyChecker(protoVersion uint32, compatibleProtoVersions []uint32, clientVersion string) handshake.CredentialChecker {
	return &noVerifyChecker{
		cred: &handshakeproto.Credentials{
			Type:          handshakeproto.CredentialsType_SkipVerify,
			Version:       protoVersion,
			ClientVersion: clientVersion,
		},
		compatibleVersions: compatibleProtoVersions,
	}
}

type noVerifyChecker struct {
	cred               *handshakeproto.Credentials
	compatibleVersions []uint32
}

func (n noVerifyChecker) MakeCredentials(remotePeerId string) *handshakeproto.Credentials {
	return n.cred
}

func (n noVerifyChecker) CheckCredential(remotePeerId string, cred *handshakeproto.Credentials) (result handshake.Result, err error) {
	if !slices.Contains(n.compatibleVersions, cred.Version) {
		err = handshake.ErrIncompatibleVersion
		return
	}
	return handshake.Result{
		ProtoVersion:  cred.Version,
		ClientVersion: cred.ClientVersion,
	}, nil
}

func newPeerSignVerifier(protoVersion uint32, compatibleProtoVersions []uint32, clientVersion string, account *accountdata.AccountKeys) handshake.CredentialChecker {
	return &peerSignVerifier{
		protoVersion:       protoVersion,
		clientVersion:      clientVersion,
		account:            account,
		compatibleVersions: compatibleProtoVersions,
	}
}

type peerSignVerifier struct {
	protoVersion       uint32
	clientVersion      string
	account            *accountdata.AccountKeys
	compatibleVersions []uint32
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
		Type:          handshakeproto.CredentialsType_SignedPeerIds,
		Payload:       payload,
		Version:       p.protoVersion,
		ClientVersion: p.clientVersion,
	}
}

func (p *peerSignVerifier) CheckCredential(remotePeerId string, cred *handshakeproto.Credentials) (result handshake.Result, err error) {
	if !slices.Contains(p.compatibleVersions, cred.Version) {
		err = handshake.ErrIncompatibleVersion
		return
	}
	if cred.Type != handshakeproto.CredentialsType_SignedPeerIds {
		err = handshake.ErrSkipVerifyNotAllowed
		return
	}
	var msg = &handshakeproto.PayloadSignedPeerIds{}
	if err = msg.Unmarshal(cred.Payload); err != nil {
		err = handshake.ErrUnexpectedPayload
		return
	}
	pubKey, err := crypto.UnmarshalEd25519PublicKeyProto(msg.Identity)
	if err != nil {
		err = handshake.ErrInvalidCredentials
		return
	}
	ok, err := pubKey.Verify([]byte((remotePeerId + p.account.PeerId)), msg.Sign)
	if err != nil {
		return
	}
	if !ok {
		err = handshake.ErrInvalidCredentials
		return
	}
	return handshake.Result{
		Identity:      msg.Identity,
		ProtoVersion:  cred.Version,
		ClientVersion: cred.ClientVersion,
	}, nil
}
