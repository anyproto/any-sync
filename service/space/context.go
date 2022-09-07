package space

import "context"

type ctxKey int

const (
	ctxKeySpaceId ctxKey = iota
)

// CtxSpaceId gets spaceId from id. If spaceId is not found in context - it returns an empty string
func CtxSpaceId(ctx context.Context) (spaceId string) {
	if val := ctx.Value(ctxKeySpaceId); val != nil {
		return val.(string)
	}
	return
}

// CtxWithSpaceId creates new context with spaceId value
func CtxWithSpaceId(ctx context.Context, spaceId string) context.Context {
	return context.WithValue(ctx, ctxKeySpaceId, spaceId)
}
