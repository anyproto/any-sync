package secureservice

import (
	"context"

	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
)

func withAdmissionVerifier(ctx context.Context, checker handshake.CredentialChecker, verifier AdmissionVerifier, required bool, networkID string) handshake.CredentialChecker {
	if verifier == nil {
		return checker
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return admissionVerifierCredentialChecker{
		CredentialChecker: checker,
		ctx:               ctx,
		verifier:          verifier,
		required:          required,
		networkID:         networkID,
	}
}

type admissionVerifierCredentialChecker struct {
	handshake.CredentialChecker
	ctx       context.Context
	verifier  AdmissionVerifier
	required  bool
	networkID string
}

func (a admissionVerifierCredentialChecker) CheckCredential(remotePeerId string, cred *handshakeproto.Credentials) (handshake.Result, error) {
	res, err := a.CredentialChecker.CheckCredential(remotePeerId, cred)
	if err != nil {
		return handshake.Result{}, err
	}
	if res.AdmissionToken == "" {
		if a.required {
			return handshake.Result{}, handshake.ErrInvalidCredentials
		}
		return res, nil
	}
	decision, err := a.verifier.VerifyAdmission(a.ctx, AdmissionRequest{
		Token:         res.AdmissionToken,
		Identity:      res.Identity,
		NetworkID:     a.networkID,
		PeerID:        remotePeerId,
		ClientVersion: res.ClientVersion,
	})
	if err != nil || !decision.Allowed {
		return handshake.Result{}, handshake.ErrInvalidCredentials
	}
	return res, nil
}
