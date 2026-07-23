package secureservice

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/testutil/accounttest"
)

func TestPeerSignVerifier_CheckCredential(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	identity1, _ := a1.SignKey.GetPublic().Marshall()
	identity2, _ := a2.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a1)
	cc2 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a2)

	c1 := a2.PeerId
	c2 := a1.PeerId

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	cr1.AdmissionToken = "token-1"
	cr2.AdmissionToken = "token-2"
	res, err := cc1.CheckCredential(c1, cr2)
	assert.NoError(t, err)
	assert.Equal(t, identity2, res.Identity)
	assert.Equal(t, "token-2", res.AdmissionToken)

	res2, err := cc2.CheckCredential(c2, cr1)
	assert.NoError(t, err)
	assert.Equal(t, identity1, res2.Identity)
	assert.Equal(t, "token-1", res2.AdmissionToken)

	_, err = cc1.CheckCredential(c1, cr1)
	assert.EqualError(t, err, handshake.ErrInvalidCredentials.Error())
}

func TestNoVerifyChecker_CheckCredentialCopiesAdmissionToken(t *testing.T) {
	cc := newNoVerifyChecker(1, []uint32{1}, "test:v1")
	cred := cc.MakeCredentials("peer-id")
	cred.AdmissionToken = "admission-token"

	res, err := cc.CheckCredential("peer-id", cred)
	require.NoError(t, err)
	assert.Equal(t, "admission-token", res.AdmissionToken)
}

func TestAdmissionTokenCheckerDoesNotMutateBaseCredentials(t *testing.T) {
	cc := newNoVerifyChecker(1, []uint32{1}, "test:v1")
	wrapped := withAdmissionToken(cc, "admission-token")

	wrappedCred := wrapped.MakeCredentials("peer-id")
	baseCred := cc.MakeCredentials("peer-id")

	assert.Equal(t, "admission-token", wrappedCred.AdmissionToken)
	assert.Empty(t, baseCred.AdmissionToken)
}

func TestAdmissionVerifierCredentialChecker_AllowsAdmissionToken(t *testing.T) {
	baseResult := handshake.Result{
		Identity:       []byte("identity"),
		ProtoVersion:   1,
		ClientVersion:  "test:v1",
		AdmissionToken: "admission-token",
	}
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: baseResult}, verifier, true, "network-1")

	res, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	require.NoError(t, err)
	assert.Equal(t, baseResult, res)
	require.Len(t, verifier.Requests(), 1)
	assert.Equal(t, AdmissionRequest{
		Token:         "admission-token",
		Identity:      []byte("identity"),
		NetworkID:     "network-1",
		PeerID:        "peer-id",
		ClientVersion: "test:v1",
	}, verifier.Requests()[0])
}

func TestAdmissionVerifierCredentialChecker_LogsAcceptedAdmission(t *testing.T) {
	logs := captureSecureServiceLogs(t)
	baseResult := handshake.Result{
		Identity:       []byte("identity"),
		ProtoVersion:   1,
		ClientVersion:  "test:v1",
		AdmissionToken: "secret-admission-token",
	}
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: baseResult}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	require.NoError(t, err)
	entries := logs.FilterMessage("admission accepted").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	assert.Equal(t, "peer-id", fields["peerId"])
	assert.Equal(t, "network-1", fields["networkId"])
	assert.Equal(t, "test:v1", fields["clientVersion"])
	assert.Equal(t, uint32(1), fields["protoVersion"])
	assert.Equal(t, true, fields["required"])
	assert.Equal(t, true, fields["hasToken"])
	assert.Equal(t, true, fields["allowed"])
	assertObservedLogsDoNotContain(t, logs, "secret-admission-token")
}

func TestAdmissionVerifierCredentialChecker_DeniesRejectedAdmissionToken(t *testing.T) {
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: false, Reason: "not admitted"},
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: handshake.Result{AdmissionToken: "admission-token"}}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	assert.Equal(t, handshake.ErrInvalidCredentials, err)
	require.Len(t, verifier.Requests(), 1)
}

func TestAdmissionVerifierCredentialChecker_LogsRejectedAdmission(t *testing.T) {
	logs := captureSecureServiceLogs(t)
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: false, Reason: "not admitted"},
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: handshake.Result{
		ProtoVersion:   1,
		ClientVersion:  "test:v1",
		AdmissionToken: "secret-admission-token",
	}}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	assert.Equal(t, handshake.ErrInvalidCredentials, err)
	entries := logs.FilterMessage("admission rejected").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	assert.Equal(t, "peer-id", fields["peerId"])
	assert.Equal(t, "network-1", fields["networkId"])
	assert.Equal(t, true, fields["required"])
	assert.Equal(t, true, fields["hasToken"])
	assert.Equal(t, false, fields["allowed"])
	assert.Equal(t, false, fields["verifierError"])
	assertObservedLogsDoNotContain(t, logs, "secret-admission-token")
}

func TestAdmissionVerifierCredentialChecker_DeniesVerifierError(t *testing.T) {
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: false, Reason: "invalid"},
		err:      ErrAdmissionInvalidToken,
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: handshake.Result{AdmissionToken: "admission-token"}}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	assert.Equal(t, handshake.ErrInvalidCredentials, err)
	require.Len(t, verifier.Requests(), 1)
}

func TestAdmissionVerifierCredentialChecker_LogsVerifierErrorWithoutSensitiveFields(t *testing.T) {
	logs := captureSecureServiceLogs(t)
	tokenSecret := "secret-token"
	reasonSecret := "secret-reason"
	errorSecret := "secret-error"
	claimSecret := "secret-claim"
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{
			Allowed: false,
			Reason:  reasonSecret,
			Claims: AdmissionClaims{
				Claims: map[string]any{"claim": claimSecret},
			},
		},
		err: errors.New(errorSecret),
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: handshake.Result{
		ProtoVersion:   1,
		ClientVersion:  "test:v1",
		AdmissionToken: tokenSecret,
	}}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	assert.Equal(t, handshake.ErrInvalidCredentials, err)
	entries := logs.FilterMessage("admission rejected").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	assert.Equal(t, true, fields["hasToken"])
	assert.Equal(t, false, fields["allowed"])
	assert.Equal(t, true, fields["verifierError"])
	assertObservedLogsDoNotContain(t, logs, tokenSecret)
	assertObservedLogsDoNotContain(t, logs, reasonSecret)
	assertObservedLogsDoNotContain(t, logs, errorSecret)
	assertObservedLogsDoNotContain(t, logs, claimSecret)
}

func TestAdmissionVerifierCredentialChecker_SkipsMissingOptionalToken(t *testing.T) {
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	baseResult := handshake.Result{ClientVersion: "test:v1"}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: baseResult}, verifier, false, "network-1")

	res, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	require.NoError(t, err)
	assert.Equal(t, baseResult, res)
	assert.Empty(t, verifier.Requests())
}

func TestAdmissionVerifierCredentialChecker_DeniesMissingRequiredToken(t *testing.T) {
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: handshake.Result{}}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	assert.Equal(t, handshake.ErrInvalidCredentials, err)
	assert.Empty(t, verifier.Requests())
}

func TestAdmissionVerifierCredentialChecker_LogsMissingRequiredToken(t *testing.T) {
	logs := captureSecureServiceLogs(t)
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{result: handshake.Result{
		ProtoVersion:  1,
		ClientVersion: "test:v1",
	}}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	assert.Equal(t, handshake.ErrInvalidCredentials, err)
	entries := logs.FilterMessage("admission rejected: missing token").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	assert.Equal(t, "peer-id", fields["peerId"])
	assert.Equal(t, "network-1", fields["networkId"])
	assert.Equal(t, true, fields["required"])
	assert.Equal(t, false, fields["hasToken"])
	assert.Equal(t, false, fields["allowed"])
	assert.Empty(t, verifier.Requests())
}

func TestAdmissionVerifierCredentialChecker_SkipsVerifierAfterBaseError(t *testing.T) {
	verifier := &recordingAdmissionVerifier{
		decision: AdmissionDecision{Allowed: true, Reason: "ok"},
	}
	checker := withAdmissionVerifier(context.Background(), staticCredentialChecker{err: handshake.ErrInvalidCredentials}, verifier, true, "network-1")

	_, err := checker.CheckCredential("peer-id", &handshakeproto.Credentials{})
	assert.Equal(t, handshake.ErrInvalidCredentials, err)
	assert.Empty(t, verifier.Requests())
}

func TestIncompatibleVersion(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	_, _ = a1.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(0, []uint32{0}, "test:v1", a1)
	cc2 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a2)

	c1 := a2.PeerId
	c2 := a1.PeerId

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	_, err := cc1.CheckCredential(c1, cr2)
	assert.EqualError(t, err, handshake.ErrIncompatibleVersion.Error())

	_, err = cc2.CheckCredential(c2, cr1)
	assert.EqualError(t, err, handshake.ErrIncompatibleVersion.Error())

	_, err = cc1.CheckCredential(c1, cr1)
	assert.EqualError(t, err, handshake.ErrInvalidCredentials.Error())
}

func TestIncompatibleVersion_Issue4423(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	identity2, _ := a2.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(1, []uint32{1}, "Linux:0.43.3/middle:v0.36.6/any-sync:v0.5.11", a1)
	cc2 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a2)

	c1 := a2.PeerId
	c2 := a1.PeerId

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	res, err := cc1.CheckCredential(c1, cr2)
	assert.NoError(t, err)
	assert.Equal(t, identity2, res.Identity)

	_, err = cc2.CheckCredential(c2, cr1)
	assert.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
}

func newTestAccData(t *testing.T) *accountdata.AccountKeys {
	as := accounttest.AccountTestService{}
	require.NoError(t, as.Init(nil))
	return as.Account()
}

var secureServiceLogCaptureMu sync.Mutex

func captureSecureServiceLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	secureServiceLogCaptureMu.Lock()
	oldLog := log
	core, logs := observer.New(zap.DebugLevel)
	log = logger.CtxLogger{Logger: zap.New(core).Named(CName)}
	t.Cleanup(func() {
		log = oldLog
		secureServiceLogCaptureMu.Unlock()
	})
	return logs
}

func assertObservedLogsDoNotContain(t *testing.T, logs *observer.ObservedLogs, secret string) {
	t.Helper()
	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, secret)
		assert.NotContains(t, fmt.Sprint(entry.ContextMap()), secret)
	}
}

type recordingAdmissionVerifier struct {
	mu       sync.Mutex
	requests []AdmissionRequest
	decision AdmissionDecision
	err      error
}

func (r *recordingAdmissionVerifier) VerifyAdmission(ctx context.Context, req AdmissionRequest) (AdmissionDecision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
	return r.decision, r.err
}

func (r *recordingAdmissionVerifier) Requests() []AdmissionRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := make([]AdmissionRequest, len(r.requests))
	copy(requests, r.requests)
	return requests
}

type staticCredentialChecker struct {
	makeCred *handshakeproto.Credentials
	result   handshake.Result
	err      error
}

func (s staticCredentialChecker) MakeCredentials(remotePeerId string) *handshakeproto.Credentials {
	return s.makeCred
}

func (s staticCredentialChecker) CheckCredential(remotePeerId string, cred *handshakeproto.Credentials) (handshake.Result, error) {
	return s.result, s.err
}
