package logger

import "go.uber.org/zap"

var DefaultLogger *zap.Logger

func init() {
	DefaultLogger, _ = zap.NewDevelopment()
}

func Default() *zap.Logger {
	return DefaultLogger
}

func NewNamed(name string, fields ...zap.Field) *zap.Logger {
	l := DefaultLogger.Named(name)
	if len(fields) > 0 {
		l = l.With(fields...)
	}
	return l
}
