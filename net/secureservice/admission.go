package secureservice

import "context"

const (
	DefaultAdmissionIdentityClaim = "anytype_identity"
	DefaultAdmissionNetworkClaim  = "network_id"
	DefaultAdmissionSubjectClaim  = "sub"
	DefaultAdmissionClockSkewSec  = 60
)

type AdmissionConfig struct {
	Enabled        bool           `yaml:"enabled"`
	Required       bool           `yaml:"required"`
	Issuer         string         `yaml:"issuer"`
	Audience       string         `yaml:"audience"`
	JWKSURL        string         `yaml:"jwksUrl"`
	RequiredClaims map[string]any `yaml:"requiredClaims"`
	IdentityClaim  string         `yaml:"identityClaim"`
	NetworkClaim   string         `yaml:"networkClaim"`
	SubjectClaim   string         `yaml:"subjectClaim"`
	ClockSkewSec   int            `yaml:"clockSkewSec"`
}

func (c AdmissionConfig) WithDefaults() AdmissionConfig {
	if c.IdentityClaim == "" {
		c.IdentityClaim = DefaultAdmissionIdentityClaim
	}
	if c.NetworkClaim == "" {
		c.NetworkClaim = DefaultAdmissionNetworkClaim
	}
	if c.SubjectClaim == "" {
		c.SubjectClaim = DefaultAdmissionSubjectClaim
	}
	if c.ClockSkewSec == 0 {
		c.ClockSkewSec = DefaultAdmissionClockSkewSec
	}
	return c
}

type AdmissionRequest struct {
	Token         string
	Identity      []byte
	NetworkID     string
	PeerID        string
	ClientVersion string
}

type AdmissionClaims struct {
	Subject   string
	Issuer    string
	Audience  []string
	NetworkID string
	Identity  []byte
	Claims    map[string]any
}

type AdmissionDecision struct {
	Allowed bool
	Reason  string
	Claims  AdmissionClaims
}

type AdmissionVerifier interface {
	VerifyAdmission(ctx context.Context, req AdmissionRequest) (AdmissionDecision, error)
}

type NoopAdmissionVerifier struct{}

func (NoopAdmissionVerifier) VerifyAdmission(ctx context.Context, req AdmissionRequest) (AdmissionDecision, error) {
	return AdmissionDecision{
		Allowed: true,
		Reason:  "admission disabled",
	}, nil
}
