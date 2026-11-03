package secureservice

import "context"

type ctxKey int

const (
	allowAccountCheck ctxKey = iota
)

type configGetter interface {
	GetSecureService() Config
}

type Config struct {
	RequireClientAuth  bool     `yaml:"requireClientAuth"`
	CompatibleVersions []uint32 `yaml:"compatibleVersions"`
	// DisableSNI stops outbound TLS handshakes from sending the dialed hostname as SNI
	DisableSNI bool `yaml:"disableSni"`
	// EnableQuicSNI makes outbound QUIC handshakes send it too, unless DisableSNI is set.
	// Off by default: censors that decrypt QUIC Initials filter by SNI, and QUIC
	// without it gives clients a path that SNI blocklists miss.
	EnableQuicSNI bool `yaml:"enableQuicSni"`
}

// CtxAllowAccountCheck upgrades the context to allow identity check on handshake
func CtxAllowAccountCheck(ctx context.Context) context.Context {
	return context.WithValue(ctx, allowAccountCheck, true)
}

// CtxIsAccountCheckAllowed checks if the "allowAccountCheck" flag is set to true in the provided context.
func CtxIsAccountCheckAllowed(ctx context.Context) bool {
	if v, ok := ctx.Value(allowAccountCheck).(bool); ok {
		return v
	}
	return false
}
