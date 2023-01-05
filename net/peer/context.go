package peer

import (
	"context"
	"errors"
	"github.com/libp2p/go-libp2p/core/sec"
	"storj.io/drpc/drpcctx"
)

type contextKey uint

const (
	contextKeyPeerId contextKey = iota
)

var ErrPeerIdNotFoundInContext = errors.New("peer id not found in context")

// CtxPeerId first tries to get peer id under our own key, but if it is not found tries to get through DRPC key
func CtxPeerId(ctx context.Context) (string, error) {
	if peerId, ok := ctx.Value(contextKeyPeerId).(string); ok {
		return peerId, nil
	}
	if conn, ok := ctx.Value(drpcctx.TransportKey{}).(sec.SecureConn); ok {
		return conn.RemotePeer().String(), nil
	}
	return "", ErrPeerIdNotFoundInContext
}

// CtxWithPeerId sets peer id in the context
func CtxWithPeerId(ctx context.Context, peerId string) context.Context {
	return context.WithValue(ctx, contextKeyPeerId, peerId)
}
