package secureservice

import (
	"context"
	stdcrypto "crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTAdmissionVerifier_VerifyAdmission(t *testing.T) {
	now := time.Unix(1700000000, 0)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	account := newTestAccData(t)
	identity, err := account.SignKey.GetPublic().Marshall()
	require.NoError(t, err)

	verifier := newTestJWTAdmissionVerifier(t, privateKey, now)
	token := signAdmissionToken(t, privateKey, map[string]any{
		"iss":              "https://issuer.example",
		"aud":              []string{"any-sync"},
		"sub":              "user-1",
		"network_id":       "network-1",
		"anytype_identity": account.SignKey.GetPublic().Account(),
		"groups":           []string{"docs-users"},
		"exp":              now.Add(time.Hour).Unix(),
		"nbf":              now.Add(-time.Minute).Unix(),
		"iat":              now.Add(-time.Minute).Unix(),
	})

	decision, err := verifier.VerifyAdmission(context.Background(), AdmissionRequest{
		Token:     token,
		Identity:  identity,
		NetworkID: "network-1",
	})

	require.NoError(t, err)
	assert.True(t, decision.Allowed)
	assert.Equal(t, "admission allowed", decision.Reason)
	assert.Equal(t, "user-1", decision.Claims.Subject)
	assert.Equal(t, "network-1", decision.Claims.NetworkID)
	assert.Equal(t, identity, decision.Claims.Identity)
}

func TestJWTAdmissionVerifier_DeniesWrongAudience(t *testing.T) {
	now := time.Unix(1700000000, 0)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	account := newTestAccData(t)
	identity, err := account.SignKey.GetPublic().Marshall()
	require.NoError(t, err)

	verifier := newTestJWTAdmissionVerifier(t, privateKey, now)
	token := signAdmissionToken(t, privateKey, map[string]any{
		"iss":              "https://issuer.example",
		"aud":              "other-audience",
		"sub":              "user-1",
		"network_id":       "network-1",
		"anytype_identity": account.SignKey.GetPublic().Account(),
		"groups":           []string{"docs-users"},
		"exp":              now.Add(time.Hour).Unix(),
	})

	decision, err := verifier.VerifyAdmission(context.Background(), AdmissionRequest{
		Token:     token,
		Identity:  identity,
		NetworkID: "network-1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAdmissionDenied))
	assert.False(t, decision.Allowed)
}

func TestJWTAdmissionVerifier_DeniesWrongIdentity(t *testing.T) {
	now := time.Unix(1700000000, 0)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	account := newTestAccData(t)
	otherAccount := newTestAccData(t)
	identity, err := account.SignKey.GetPublic().Marshall()
	require.NoError(t, err)

	verifier := newTestJWTAdmissionVerifier(t, privateKey, now)
	token := signAdmissionToken(t, privateKey, map[string]any{
		"iss":              "https://issuer.example",
		"aud":              "any-sync",
		"sub":              "user-1",
		"network_id":       "network-1",
		"anytype_identity": otherAccount.SignKey.GetPublic().Account(),
		"groups":           []string{"docs-users"},
		"exp":              now.Add(time.Hour).Unix(),
	})

	decision, err := verifier.VerifyAdmission(context.Background(), AdmissionRequest{
		Token:     token,
		Identity:  identity,
		NetworkID: "network-1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAdmissionDenied))
	assert.False(t, decision.Allowed)
}

func TestJWTAdmissionVerifier_DeniesTamperedToken(t *testing.T) {
	now := time.Unix(1700000000, 0)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	account := newTestAccData(t)
	identity, err := account.SignKey.GetPublic().Marshall()
	require.NoError(t, err)

	verifier := newTestJWTAdmissionVerifier(t, privateKey, now)
	token := signAdmissionToken(t, privateKey, map[string]any{
		"iss":              "https://issuer.example",
		"aud":              "any-sync",
		"sub":              "user-1",
		"network_id":       "network-1",
		"anytype_identity": account.SignKey.GetPublic().Account(),
		"groups":           []string{"docs-users"},
		"exp":              now.Add(time.Hour).Unix(),
	})
	tokenParts := strings.Split(token, ".")
	require.Len(t, tokenParts, 3)
	tokenParts[2] = base64.RawURLEncoding.EncodeToString([]byte("bad-signature"))

	decision, err := verifier.VerifyAdmission(context.Background(), AdmissionRequest{
		Token:     strings.Join(tokenParts, "."),
		Identity:  identity,
		NetworkID: "network-1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAdmissionDenied))
	assert.False(t, decision.Allowed)
}

func TestJWTAdmissionVerifier_DeniesExpiredToken(t *testing.T) {
	now := time.Unix(1700000000, 0)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	account := newTestAccData(t)
	identity, err := account.SignKey.GetPublic().Marshall()
	require.NoError(t, err)

	verifier := newTestJWTAdmissionVerifier(t, privateKey, now)
	token := signAdmissionToken(t, privateKey, map[string]any{
		"iss":              "https://issuer.example",
		"aud":              "any-sync",
		"sub":              "user-1",
		"network_id":       "network-1",
		"anytype_identity": account.SignKey.GetPublic().Account(),
		"groups":           []string{"docs-users"},
		"exp":              now.Add(-2 * time.Minute).Unix(),
	})

	decision, err := verifier.VerifyAdmission(context.Background(), AdmissionRequest{
		Token:     token,
		Identity:  identity,
		NetworkID: "network-1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAdmissionDenied))
	assert.False(t, decision.Allowed)
}

func newTestJWTAdmissionVerifier(t *testing.T, privateKey *rsa.PrivateKey, now time.Time) *jwtAdmissionVerifier {
	verifier, err := newJWTAdmissionVerifier(AdmissionConfig{
		Issuer:         "https://issuer.example",
		Audience:       "any-sync",
		RequiredClaims: map[string]any{"groups": "docs-users"},
	}, rsaJWKS(t, &privateKey.PublicKey, "test-key"), func() time.Time { return now })
	require.NoError(t, err)
	return verifier
}

func signAdmissionToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]any) string {
	header := map[string]any{
		"alg": "RS256",
		"kid": "test-key",
	}
	encodedHeader := encodeJWTPart(t, header)
	encodedClaims := encodeJWTPart(t, claims)
	signingInput := encodedHeader + "." + encodedClaims
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, stdcrypto.SHA256, digest[:])
	require.NoError(t, err)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func rsaJWKS(t *testing.T, publicKey *rsa.PublicKey, keyID string) []byte {
	return mustJSON(t, map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": keyID,
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
			},
		},
	})
}

func encodeJWTPart(t *testing.T, value any) string {
	return base64.RawURLEncoding.EncodeToString(mustJSON(t, value))
}

func mustJSON(t *testing.T, value any) []byte {
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
