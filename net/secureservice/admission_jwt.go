package secureservice

import (
	"bytes"
	"context"
	stdcrypto "crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	anycrypto "github.com/anyproto/any-sync/util/crypto"
)

var (
	// ErrAdmissionDenied is returned when an admission request is rejected.
	ErrAdmissionDenied = errors.New("admission denied")
	// ErrAdmissionInvalidConfig is returned when JWT admission verifier configuration is incomplete or invalid.
	ErrAdmissionInvalidConfig = errors.New("invalid admission config")
	// ErrAdmissionInvalidToken is returned when an admission token is malformed or fails validation.
	ErrAdmissionInvalidToken = errors.New("invalid admission token")
)

type jwtAdmissionVerifier struct {
	conf AdmissionConfig
	keys map[string]jwtPublicKey
	now  func() time.Time
}

type jwtPublicKey struct {
	algorithm string
	key       any
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	N         string `json:"n"`
	E         string `json:"e"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
}

// NewJWTAdmissionVerifier creates an AdmissionVerifier that validates JWTs against a static JWKS document.
func NewJWTAdmissionVerifier(conf AdmissionConfig, jwks []byte) (AdmissionVerifier, error) {
	return newJWTAdmissionVerifier(conf, jwks, time.Now)
}

func newJWTAdmissionVerifier(conf AdmissionConfig, jwks []byte, now func() time.Time) (*jwtAdmissionVerifier, error) {
	conf = conf.WithDefaults()
	if conf.Issuer == "" || conf.Audience == "" || conf.IdentityClaim == "" || conf.NetworkClaim == "" || conf.SubjectClaim == "" {
		return nil, ErrAdmissionInvalidConfig
	}
	keys, err := parseJWKS(jwks)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, ErrAdmissionInvalidConfig
	}
	if now == nil {
		now = time.Now
	}
	return &jwtAdmissionVerifier{
		conf: conf,
		keys: keys,
		now:  now,
	}, nil
}

func (v *jwtAdmissionVerifier) VerifyAdmission(ctx context.Context, req AdmissionRequest) (AdmissionDecision, error) {
	if err := ctx.Err(); err != nil {
		return AdmissionDecision{Allowed: false, Reason: err.Error()}, err
	}
	if req.Token == "" || len(req.Identity) == 0 || req.NetworkID == "" {
		return v.deny("missing admission token, identity, or network id", ErrAdmissionInvalidToken)
	}

	header, claims, signingInput, signature, err := parseJWT(req.Token)
	if err != nil {
		return v.deny("malformed admission token", err)
	}
	key, ok := v.keys[header.KeyID]
	if !ok {
		return v.deny("unknown signing key", ErrAdmissionInvalidToken)
	}
	if err = verifyJWTSignature(header.Algorithm, key, signingInput, signature); err != nil {
		return v.deny("invalid admission token signature", err)
	}

	decisionClaims, err := v.validateClaims(req, claims)
	if err != nil {
		return v.deny(err.Error(), err)
	}
	return AdmissionDecision{
		Allowed: true,
		Reason:  "admission allowed",
		Claims:  decisionClaims,
	}, nil
}

func (v *jwtAdmissionVerifier) deny(reason string, err error) (AdmissionDecision, error) {
	return AdmissionDecision{Allowed: false, Reason: reason}, errors.Join(ErrAdmissionDenied, err)
}

func parseJWT(token string) (jwtHeader, map[string]any, []byte, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, nil, nil, nil, ErrAdmissionInvalidToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, nil, nil, nil, ErrAdmissionInvalidToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, nil, nil, nil, ErrAdmissionInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, nil, nil, nil, ErrAdmissionInvalidToken
	}

	var header jwtHeader
	if err = decodeJSON(headerBytes, &header); err != nil {
		return jwtHeader{}, nil, nil, nil, ErrAdmissionInvalidToken
	}
	if header.Algorithm == "" || header.KeyID == "" {
		return jwtHeader{}, nil, nil, nil, ErrAdmissionInvalidToken
	}

	claims := map[string]any{}
	if err = decodeJSON(payloadBytes, &claims); err != nil {
		return jwtHeader{}, nil, nil, nil, ErrAdmissionInvalidToken
	}
	return header, claims, []byte(parts[0] + "." + parts[1]), signature, nil
}

func decodeJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

func parseJWKS(jwks []byte) (map[string]jwtPublicKey, error) {
	var set jsonWebKeySet
	if err := decodeJSON(jwks, &set); err != nil {
		return nil, ErrAdmissionInvalidConfig
	}
	keys := make(map[string]jwtPublicKey, len(set.Keys))
	for _, key := range set.Keys {
		if key.KeyID == "" || key.Use == "enc" {
			continue
		}
		parsed, err := parseJWK(key)
		if err != nil {
			return nil, err
		}
		keys[key.KeyID] = parsed
	}
	return keys, nil
}

func parseJWK(key jsonWebKey) (jwtPublicKey, error) {
	switch key.KeyType {
	case "RSA":
		return parseRSAJWK(key)
	case "EC":
		return parseECJWK(key)
	case "OKP":
		return parseOKPJWK(key)
	default:
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
}

func parseRSAJWK(key jsonWebKey) (jwtPublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	e, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	exponent := int(new(big.Int).SetBytes(e).Int64())
	if exponent == 0 || len(n) == 0 {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	return jwtPublicKey{
		algorithm: key.Algorithm,
		key: &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: exponent,
		},
	}, nil
}

func parseECJWK(key jsonWebKey) (jwtPublicKey, error) {
	x, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	y, err := base64.RawURLEncoding.DecodeString(key.Y)
	if err != nil {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	curve := curveForJWK(key.Curve)
	xCoord := new(big.Int).SetBytes(x)
	yCoord := new(big.Int).SetBytes(y)
	if curve == nil || len(x) == 0 || len(y) == 0 || !curve.IsOnCurve(xCoord, yCoord) {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	return jwtPublicKey{
		algorithm: key.Algorithm,
		key: &ecdsa.PublicKey{
			Curve: curve,
			X:     xCoord,
			Y:     yCoord,
		},
	}, nil
}

func curveForJWK(curve string) elliptic.Curve {
	switch curve {
	case "P-256":
		return elliptic.P256()
	case "P-384":
		return elliptic.P384()
	case "P-521":
		return elliptic.P521()
	default:
		return nil
	}
}

func parseOKPJWK(key jsonWebKey) (jwtPublicKey, error) {
	if key.Curve != "Ed25519" {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	x, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil || len(x) != ed25519.PublicKeySize {
		return jwtPublicKey{}, ErrAdmissionInvalidConfig
	}
	return jwtPublicKey{
		algorithm: key.Algorithm,
		key:       ed25519.PublicKey(x),
	}, nil
}

func verifyJWTSignature(algorithm string, key jwtPublicKey, signingInput []byte, signature []byte) error {
	if key.algorithm != "" && key.algorithm != algorithm {
		return ErrAdmissionInvalidToken
	}
	switch algorithm {
	case "RS256":
		digest := sha256.Sum256(signingInput)
		return verifyRSA(key.key, stdcrypto.SHA256, digest[:], signature)
	case "RS384":
		digest := sha512.Sum384(signingInput)
		return verifyRSA(key.key, stdcrypto.SHA384, digest[:], signature)
	case "RS512":
		digest := sha512.Sum512(signingInput)
		return verifyRSA(key.key, stdcrypto.SHA512, digest[:], signature)
	case "ES256":
		digest := sha256.Sum256(signingInput)
		return verifyECDSA(key.key, digest[:], signature, 32)
	case "ES384":
		digest := sha512.Sum384(signingInput)
		return verifyECDSA(key.key, digest[:], signature, 48)
	case "ES512":
		digest := sha512.Sum512(signingInput)
		return verifyECDSA(key.key, digest[:], signature, 66)
	case "EdDSA":
		pub, ok := key.key.(ed25519.PublicKey)
		if !ok || !ed25519.Verify(pub, signingInput, signature) {
			return ErrAdmissionInvalidToken
		}
		return nil
	default:
		return ErrAdmissionInvalidToken
	}
}

func verifyRSA(key any, hash stdcrypto.Hash, digest []byte, signature []byte) error {
	pub, ok := key.(*rsa.PublicKey)
	if !ok {
		return ErrAdmissionInvalidToken
	}
	if err := rsa.VerifyPKCS1v15(pub, hash, digest, signature); err != nil {
		return ErrAdmissionInvalidToken
	}
	return nil
}

func verifyECDSA(key any, digest []byte, signature []byte, keyBytes int) error {
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok || len(signature) != keyBytes*2 {
		return ErrAdmissionInvalidToken
	}
	r := new(big.Int).SetBytes(signature[:keyBytes])
	s := new(big.Int).SetBytes(signature[keyBytes:])
	if !ecdsa.Verify(pub, digest, r, s) {
		return ErrAdmissionInvalidToken
	}
	return nil
}

func (v *jwtAdmissionVerifier) validateClaims(req AdmissionRequest, claims map[string]any) (AdmissionClaims, error) {
	now := v.now()
	if err := validateStringClaim(claims, "iss", v.conf.Issuer); err != nil {
		return AdmissionClaims{}, err
	}
	audience, err := validateAudienceClaim(claims, v.conf.Audience)
	if err != nil {
		return AdmissionClaims{}, err
	}
	if err = validateExpiration(claims, now, time.Duration(v.conf.ClockSkewSec)*time.Second); err != nil {
		return AdmissionClaims{}, err
	}
	if err = validateStringClaim(claims, v.conf.NetworkClaim, req.NetworkID); err != nil {
		return AdmissionClaims{}, err
	}
	accountID, err := accountIDFromIdentity(req.Identity)
	if err != nil {
		return AdmissionClaims{}, ErrAdmissionInvalidToken
	}
	if err = validateStringClaim(claims, v.conf.IdentityClaim, accountID); err != nil {
		return AdmissionClaims{}, err
	}
	subject, err := stringClaim(claims, v.conf.SubjectClaim)
	if err != nil || subject == "" {
		return AdmissionClaims{}, ErrAdmissionInvalidToken
	}
	for name, expected := range v.conf.RequiredClaims {
		actual, ok := claims[name]
		if !ok || !claimMatches(actual, expected) {
			return AdmissionClaims{}, fmt.Errorf("required claim %q is missing or invalid: %w", name, ErrAdmissionInvalidToken)
		}
	}
	return AdmissionClaims{
		Subject:   subject,
		Issuer:    v.conf.Issuer,
		Audience:  audience,
		NetworkID: req.NetworkID,
		Identity:  req.Identity,
		Claims:    claims,
	}, nil
}

func accountIDFromIdentity(identity []byte) (string, error) {
	pub, err := anycrypto.UnmarshalEd25519PublicKeyProto(identity)
	if err != nil {
		return "", err
	}
	return pub.Account(), nil
}

func validateStringClaim(claims map[string]any, name string, expected string) error {
	actual, err := stringClaim(claims, name)
	if err != nil || actual != expected {
		return ErrAdmissionInvalidToken
	}
	return nil
}

func stringClaim(claims map[string]any, name string) (string, error) {
	value, ok := claims[name].(string)
	if !ok {
		return "", ErrAdmissionInvalidToken
	}
	return value, nil
}

func validateAudienceClaim(claims map[string]any, expected string) ([]string, error) {
	audience := stringListClaim(claims["aud"])
	for _, value := range audience {
		if value == expected {
			return audience, nil
		}
	}
	return nil, ErrAdmissionInvalidToken
}

func stringListClaim(value any) []string {
	switch value := value.(type) {
	case string:
		return []string{value}
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

func validateExpiration(claims map[string]any, now time.Time, skew time.Duration) error {
	exp, ok, err := numericDateClaim(claims, "exp")
	if err != nil || !ok || now.After(exp.Add(skew)) {
		return ErrAdmissionInvalidToken
	}
	if nbf, ok, err := numericDateClaim(claims, "nbf"); err != nil || ok && now.Add(skew).Before(nbf) {
		return ErrAdmissionInvalidToken
	}
	if iat, ok, err := numericDateClaim(claims, "iat"); err != nil || ok && now.Add(skew).Before(iat) {
		return ErrAdmissionInvalidToken
	}
	return nil
}

func numericDateClaim(claims map[string]any, name string) (time.Time, bool, error) {
	value, ok := claims[name]
	if !ok {
		return time.Time{}, false, nil
	}
	seconds, err := int64ClaimValue(value)
	if err != nil {
		return time.Time{}, false, err
	}
	return time.Unix(seconds, 0), true, nil
}

func int64ClaimValue(value any) (int64, error) {
	switch value := value.(type) {
	case json.Number:
		return value.Int64()
	case float64:
		return int64(value), nil
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	default:
		return 0, ErrAdmissionInvalidToken
	}
}

func claimMatches(actual any, expected any) bool {
	if values, ok := actual.([]any); ok {
		for _, value := range values {
			if claimMatches(value, expected) {
				return true
			}
		}
		return false
	}
	switch expected := expected.(type) {
	case string:
		actual, ok := actual.(string)
		return ok && actual == expected
	case bool:
		actual, ok := actual.(bool)
		return ok && actual == expected
	case int:
		return claimNumberMatches(actual, int64(expected))
	case int64:
		return claimNumberMatches(actual, expected)
	case json.Number:
		expectedInt, err := expected.Int64()
		return err == nil && claimNumberMatches(actual, expectedInt)
	default:
		return actual == expected
	}
}

func claimNumberMatches(actual any, expected int64) bool {
	actualInt, err := int64ClaimValue(actual)
	return err == nil && actualInt == expected
}
