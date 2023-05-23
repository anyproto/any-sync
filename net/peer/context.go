package peer

import (
	"context"
	"errors"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/libp2p/go-libp2p/core/sec"
	"storj.io/drpc/drpcctx"
)

type contextKey uint

const (
	contextKeyPeerId contextKey = iota
	contextKeyIdentity
	contextKeyPeerAddr
)

var (
	ErrPeerIdNotFoundInContext   = errors.New("peer id not found in context")
	ErrIdentityNotFoundInContext = errors.New("identity not found in context")
)

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

// CtxPeerAddr returns peer address
func CtxPeerAddr(ctx context.Context) string {
	if p, ok := ctx.Value(contextKeyPeerAddr).(string); ok {
		return p
	}
	return ""
}

// CtxWithPeerAddr sets peer address to the context
func CtxWithPeerAddr(ctx context.Context, addr string) context.Context {
	return context.WithValue(ctx, contextKeyPeerAddr, addr)
}

// CtxIdentity returns identity from context
func CtxIdentity(ctx context.Context) ([]byte, error) {
	if identity, ok := ctx.Value(contextKeyIdentity).([]byte); ok {
		return identity, nil
	}
	return nil, ErrIdentityNotFoundInContext
}

// CtxPubKey returns identity unmarshalled from proto in crypto.PubKey model
func CtxPubKey(ctx context.Context) (crypto.PubKey, error) {
	if identity, ok := ctx.Value(contextKeyIdentity).([]byte); ok {
		return crypto.UnmarshalEd25519PublicKeyProto(identity)
	}
	return nil, ErrIdentityNotFoundInContext
}

// CtxWithIdentity sets identity in the context
func CtxWithIdentity(ctx context.Context, identity []byte) context.Context {
	return context.WithValue(ctx, contextKeyIdentity, identity)
}
