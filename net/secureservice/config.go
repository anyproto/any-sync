package secureservice

import "context"

type ctxKey int

const (
	allowAccountCheck ctxKey = iota
	outboundAdmissionToken
	remoteAdmissionToken
)

type configGetter interface {
	GetSecureService() Config
}

type Config struct {
	RequireClientAuth  bool            `yaml:"requireClientAuth"`
	CompatibleVersions []uint32        `yaml:"compatibleVersions"`
	Admission          AdmissionConfig `yaml:"admission"`
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

// CtxWithOutboundAdmissionToken stores the local admission token to send during the handshake.
func CtxWithOutboundAdmissionToken(ctx context.Context, token string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, outboundAdmissionToken, token)
}

// CtxOutboundAdmissionToken returns the local admission token to send during the handshake.
func CtxOutboundAdmissionToken(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(outboundAdmissionToken).(string); ok {
		return v
	}
	return ""
}

func ctxWithRemoteAdmissionToken(ctx context.Context, token string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, remoteAdmissionToken, token)
}

// CtxRemoteAdmissionToken returns the remote admission token received during the handshake.
func CtxRemoteAdmissionToken(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(remoteAdmissionToken).(string); ok {
		return v
	}
	return ""
}
