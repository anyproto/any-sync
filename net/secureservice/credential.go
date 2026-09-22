package secureservice

import (
	"bufio"
	"os"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/util/crypto"
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
	// Hotfix for a bad version
	if strings.Contains(cred.ClientVersion, "middle:v0.36.6") {
		err = handshake.ErrIncompatibleVersion
		return
	}
	return handshake.Result{
		ProtoVersion:  cred.Version,
		ClientVersion: cred.ClientVersion,
	}, nil
}

func newPeerSignVerifier(protoVersion uint32, compatibleProtoVersions []uint32, clientVersion string, account *accountdata.AccountKeys) (handshake.CredentialChecker, error) {
	peerSignVerifier := &peerSignVerifier{
		protoVersion:       protoVersion,
		clientVersion:      clientVersion,
		account:            account,
		compatibleVersions: compatibleProtoVersions,
	}

	path, ok := os.LookupEnv("ALLOWED_PEERS")
	if !ok {
		peerSignVerifier.allowedPeerPubKeys = nil
		return peerSignVerifier, nil
	}
	path = strings.TrimSpace(path)

	file, err := os.Open(path)
	if err != nil {
		return peerSignVerifier, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	pubKeys := make(map[[32]byte]crypto.PubKey, 5)
	for scanner.Scan() {
		line := scanner.Text()
		pub, err := crypto.UnmarshalEd25519PublicKey([]byte(line))
		if err != nil {
			return peerSignVerifier, err
		}

		raw, err := pub.Raw()
		if err != nil {
			return peerSignVerifier, err
		}

		pubKeys[[32]byte(raw)] = pub
	}

	if err = scanner.Err(); err != nil {
		return peerSignVerifier, err
	}

	peerSignVerifier.allowedPeerPubKeys = pubKeys
	return peerSignVerifier, nil
}

type peerSignVerifier struct {
	protoVersion       uint32
	clientVersion      string
	account            *accountdata.AccountKeys
	compatibleVersions []uint32

	allowedPeerPubKeys map[[32]byte]crypto.PubKey
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
	payload, _ := msg.MarshalVT()
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
	if err = msg.UnmarshalVT(cred.Payload); err != nil {
		err = handshake.ErrUnexpectedPayload
		return
	}
	pubKey, err := crypto.UnmarshalEd25519PublicKeyProto(msg.Identity)
	if err != nil {
		err = handshake.ErrInvalidCredentials
		return
	}

	if err = p.validateAllowList(pubKey); err != nil {
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
	// Hotfix for a bad version
	if strings.Contains(cred.ClientVersion, "middle:v0.36.6") {
		err = handshake.ErrIncompatibleVersion
		return
	}
	return handshake.Result{
		Identity:      msg.Identity,
		ProtoVersion:  cred.Version,
		ClientVersion: cred.ClientVersion,
	}, nil
}

func (p *peerSignVerifier) validateAllowList(pubKey crypto.PubKey) (err error) {
	err = nil

	if p.allowedPeerPubKeys == nil {
		return
	}

	var raw []byte
	raw, err = pubKey.Raw()
	if err != nil {
		err = handshake.ErrInvalidCredentials
		return
	}

	_, ok := p.allowedPeerPubKeys[[32]byte(raw)]
	if !ok {
		err = handshake.ErrInvalidCredentials
		return
	}

	return
}
