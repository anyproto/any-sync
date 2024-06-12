package handshake

import (
	"errors"

	"github.com/anyproto/any-sync/net/internal/secureservice/handshake/handshakeproto"
)

type HandshakeError struct {
	Err error
	E   handshakeproto.Error
}

func (he HandshakeError) Error() string {
	if he.Err != nil {
		return he.Err.Error()
	}
	return he.E.String()
}

var (
	ErrUnexpectedPayload       = HandshakeError{E: handshakeproto.Error_UnexpectedPayload}
	ErrDeadlineExceeded        = HandshakeError{E: handshakeproto.Error_DeadlineExceeded}
	ErrInvalidCredentials      = HandshakeError{E: handshakeproto.Error_InvalidCredentials}
	ErrPeerDeclinedCredentials = HandshakeError{Err: errors.New("remote peer declined the credentials")}
	ErrSkipVerifyNotAllowed    = HandshakeError{E: handshakeproto.Error_SkipVerifyNotAllowed}
	ErrUnexpected              = HandshakeError{E: handshakeproto.Error_Unexpected}

	ErrIncompatibleVersion     = HandshakeError{E: handshakeproto.Error_IncompatibleVersion}
	ErrIncompatibleProto       = HandshakeError{E: handshakeproto.Error_IncompatibleProto}
	ErrRemoteIncompatibleProto = HandshakeError{Err: errors.New("remote peer declined the proto")}

	ErrGotUnexpectedMessage = errors.New("go not a handshake message")
)

type CredentialChecker interface {
	MakeCredentials(remotePeerId string) *handshakeproto.Credentials
	CheckCredential(remotePeerId string, cred *handshakeproto.Credentials) (result Result, err error)
}

type Result struct {
	Identity      []byte
	ProtoVersion  uint32
	ClientVersion string
}
