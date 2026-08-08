package secureservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAdmissionConfig_WithDefaults(t *testing.T) {
	conf := AdmissionConfig{}.WithDefaults()

	assert.False(t, conf.Enabled)
	assert.False(t, conf.Required)
	assert.Equal(t, DefaultAdmissionIdentityClaim, conf.IdentityClaim)
	assert.Equal(t, DefaultAdmissionNetworkClaim, conf.NetworkClaim)
	assert.Equal(t, DefaultAdmissionSubjectClaim, conf.SubjectClaim)
	assert.Equal(t, DefaultAdmissionClockSkewSec, conf.ClockSkewSec)
}

func TestAdmissionConfig_WithDefaultsPreservesConfiguredValues(t *testing.T) {
	conf := AdmissionConfig{
		Enabled:       true,
		Required:      true,
		IdentityClaim: "identity",
		NetworkClaim:  "network",
		SubjectClaim:  "subject",
		ClockSkewSec:  120,
	}.WithDefaults()

	assert.True(t, conf.Enabled)
	assert.True(t, conf.Required)
	assert.Equal(t, "identity", conf.IdentityClaim)
	assert.Equal(t, "network", conf.NetworkClaim)
	assert.Equal(t, "subject", conf.SubjectClaim)
	assert.Equal(t, 120, conf.ClockSkewSec)
}

func TestAdmissionConfig_YAML(t *testing.T) {
	var conf Config
	err := yaml.Unmarshal([]byte(`
admission:
  enabled: true
  required: true
  issuer: https://issuer.example
  audience: any-sync
  jwksUrl: https://issuer.example/.well-known/jwks.json
  requiredClaims:
    group: docs-users
  identityClaim: anytype_id
  networkClaim: sync_network
  subjectClaim: user
  clockSkewSec: 30
`), &conf)
	require.NoError(t, err)

	assert.True(t, conf.Admission.Enabled)
	assert.True(t, conf.Admission.Required)
	assert.Equal(t, "https://issuer.example", conf.Admission.Issuer)
	assert.Equal(t, "any-sync", conf.Admission.Audience)
	assert.Equal(t, "https://issuer.example/.well-known/jwks.json", conf.Admission.JWKSURL)
	assert.Equal(t, map[string]any{"group": "docs-users"}, conf.Admission.RequiredClaims)
	assert.Equal(t, "anytype_id", conf.Admission.IdentityClaim)
	assert.Equal(t, "sync_network", conf.Admission.NetworkClaim)
	assert.Equal(t, "user", conf.Admission.SubjectClaim)
	assert.Equal(t, 30, conf.Admission.ClockSkewSec)
}

func TestNoopAdmissionVerifier_AllowsRequests(t *testing.T) {
	verifier := NoopAdmissionVerifier{}
	decision, err := verifier.VerifyAdmission(context.Background(), AdmissionRequest{})

	require.NoError(t, err)
	assert.True(t, decision.Allowed)
	assert.Equal(t, "admission disabled", decision.Reason)
}
