package peerservice

import "context"

type ctxKey uint

const ctxKeyGlobalDial ctxKey = iota

// CtxWithGlobalDial lets Dial use the iroh transport for the peer. A relay
// dial can take the whole dial timeout, so iroh addresses are opt-in: a
// caller without this flag never leaves the LAN/node transports, and a
// peer that only has an iroh address answers with ErrAddrsNotFound.
func CtxWithGlobalDial(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyGlobalDial, true)
}

// CtxIsGlobalDial reports whether ctx allows iroh dials.
func CtxIsGlobalDial(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyGlobalDial).(bool)
	return v
}
