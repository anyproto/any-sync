package logger

import (
	"go.uber.org/zap"
	"sync"
)

var (
	mu            sync.Mutex
	defaultLogger *zap.Logger
	levels        = make(map[string]zap.AtomicLevel)
	loggers       = make(map[string]CtxLogger)
)

func init() {
	defaultLogger, _ = zap.NewDevelopment()
	zap.NewProduction()
}

func SetDefault(l *zap.Logger) {
	mu.Lock()
	defer mu.Unlock()
	*defaultLogger = *l
	for name, l := range loggers {
		*l.Logger = *defaultLogger.Named(name)
	}
}

func SetNamedLevels(l map[string]zap.AtomicLevel) {
	mu.Lock()
	defer mu.Unlock()
	levels = l
}

func Default() *zap.Logger {
	mu.Lock()
	defer mu.Unlock()
	return defaultLogger
}

func NewNamed(name string, fields ...zap.Field) CtxLogger {
	mu.Lock()
	defer mu.Unlock()
	l := defaultLogger.Named(name)
	if len(fields) > 0 {
		l = l.With(fields...)
	}
	ctxL := CtxLogger{l}
	loggers[name] = ctxL
	return ctxL
}
