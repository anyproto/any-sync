package logger

import (
	"go.uber.org/zap"
	"sync"
)

var (
	mu            sync.Mutex
	defaultLogger *zap.Logger
	levels        = make(map[string]zap.AtomicLevel)
	loggers       = make(map[string]*zap.Logger)
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
		*l = *defaultLogger.Named(name)
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

func NewNamed(name string, fields ...zap.Field) *zap.Logger {
	mu.Lock()
	defer mu.Unlock()
	l := defaultLogger.Named(name)
	if len(fields) > 0 {
		l = l.With(fields...)
	}
	loggers[name] = l
	return l
}
