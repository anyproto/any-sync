package peerservice

import "context"

type ctxKey uint

const ctxKeyGlobalDial ctxKey = iota

// CtxWithGlobalDial lets Dial use the iroh transport for the peer. A relay
// dial can take the whole dial timeout, so iroh addresses are opt-in: a
// caller without this flag never leaves the LAN/node transports, and a
// peer that only has an iroh address answers with ErrAddrsNotFound.
//
// The pool shares one in-flight load per peer between concurrent callers,
// so a global dial that races a plain Get for the same peer can receive that
// Get's ErrAddrsNotFound; callers retry on their own schedule.
func CtxWithGlobalDial(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyGlobalDial, true)
}

func ctxIsGlobalDial(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyGlobalDial).(bool)
	return v
}
