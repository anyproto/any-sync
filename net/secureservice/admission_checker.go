package secureservice

import (
	"context"

	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"go.uber.org/zap"
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
			log.InfoCtx(a.ctx, "admission rejected: missing token", a.logFields(remotePeerId, res,
				zap.Bool("hasToken", false),
				zap.Bool("allowed", false),
			)...)
			return handshake.Result{}, handshake.ErrInvalidCredentials
		}
		log.DebugCtx(a.ctx, "admission token absent", a.logFields(remotePeerId, res,
			zap.Bool("hasToken", false),
		)...)
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
		log.InfoCtx(a.ctx, "admission rejected", a.logFields(remotePeerId, res,
			zap.Bool("hasToken", true),
			zap.Bool("allowed", false),
			zap.Bool("verifierError", err != nil),
		)...)
		return handshake.Result{}, handshake.ErrInvalidCredentials
	}
	log.DebugCtx(a.ctx, "admission accepted", a.logFields(remotePeerId, res,
		zap.Bool("hasToken", true),
		zap.Bool("allowed", true),
	)...)
	return res, nil
}

func (a admissionVerifierCredentialChecker) logFields(remotePeerId string, res handshake.Result, fields ...zap.Field) []zap.Field {
	base := []zap.Field{
		zap.String("peerId", remotePeerId),
		zap.String("networkId", a.networkID),
		zap.String("clientVersion", res.ClientVersion),
		zap.Uint32("protoVersion", res.ProtoVersion),
		zap.Bool("required", a.required),
	}
	return append(base, fields...)
}
