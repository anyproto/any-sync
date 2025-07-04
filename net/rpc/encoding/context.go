package encoding

import "context"

type ctxKey uint32

const (
	ctxKeySnappy ctxKey = iota
)

func CtxWithSnappy(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeySnappy, true)
}

func CtxIsSnappy(ctx context.Context) bool {
	isSnappy, _ := ctx.Value(ctxKeySnappy).(bool)
	return isSnappy
}
