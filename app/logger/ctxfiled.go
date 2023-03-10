package logger

import (
	"context"
	"go.uber.org/zap"
)

type ctxKey uint

const (
	ctxKeyFields ctxKey = iota
)

func WithCtx(ctx context.Context, l *zap.Logger) *zap.Logger {
	return l.With(CtxGetFields(ctx)...)
}

func CtxWithFields(ctx context.Context, fields ...zap.Field) context.Context {
	existingFields := CtxGetFields(ctx)
	if existingFields != nil {
		existingFields = append(existingFields, fields...)
	}
	return context.WithValue(ctx, ctxKeyFields, fields)
}

func CtxGetFields(ctx context.Context) (fields []zap.Field) {
	if v := ctx.Value(ctxKeyFields); v != nil {
		return v.([]zap.Field)
	}
	return
}

type CtxLogger struct {
	*zap.Logger
	name string
}

func (cl CtxLogger) DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	cl.Logger.Debug(msg, append(CtxGetFields(ctx), fields...)...)
}

func (cl CtxLogger) InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	cl.Logger.Info(msg, append(CtxGetFields(ctx), fields...)...)
}

func (cl CtxLogger) WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	cl.Logger.Warn(msg, append(CtxGetFields(ctx), fields...)...)
}

func (cl CtxLogger) ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	cl.Logger.Error(msg, append(CtxGetFields(ctx), fields...)...)
}

func (cl CtxLogger) With(fields ...zap.Field) CtxLogger {
	return CtxLogger{cl.Logger.With(fields...), cl.name}
}

func (cl CtxLogger) Sugar() *zap.SugaredLogger {
	return NewNamedSugared(cl.name)
}
