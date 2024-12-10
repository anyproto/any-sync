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
	RequireClientAuth bool `yaml:"requireClientAuth"`
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
