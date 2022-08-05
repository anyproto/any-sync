package secure

import (
	"context"
	"errors"
	"github.com/libp2p/go-libp2p-core/sec"
)

var (
	ErrSecureConnNotFoundInContext = errors.New("secure connection not found in context")
)

type contextKey uint

const (
	contextKeySecureConn contextKey = iota
)

func CtxSecureConn(ctx context.Context) (sec.SecureConn, error) {
	if conn, ok := ctx.Value(contextKeySecureConn).(sec.SecureConn); ok {
		return conn, nil
	}
	return nil, ErrSecureConnNotFoundInContext
}

func ctxWithSecureConn(ctx context.Context, conn sec.SecureConn) context.Context {
	return context.WithValue(ctx, contextKeySecureConn, conn)
}
