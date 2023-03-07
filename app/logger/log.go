package logger

import (
	"sync"

	"github.com/gobwas/glob"
	"go.uber.org/zap"
)

var (
	mu           sync.Mutex
	logger       *zap.Logger
	loggerConfig zap.Config
	namedLevels  = make(map[string]zap.AtomicLevel)
	namedGlobs   = make(map[string]glob.Glob)
	namedLoggers = make(map[string]CtxLogger)
)

func init() {
	loggerConfig = zap.NewDevelopmentConfig()
	logger, _ = loggerConfig.Build()
}

// SetDefault replaces the default logger
// you need to call SetNamedLevels after in case you have named loggers,
// otherwise they will use the old logger
func SetDefault(l *zap.Logger) {
	mu.Lock()
	defer mu.Unlock()
	*logger = *l
}

// SetNamedLevels sets the namedLevels for named loggers
// it also supports glob patterns for names, like "app*"
// can be racy in case there are existing named loggers
// so consider to call only once at the beginning
func SetNamedLevels(l map[string]zap.AtomicLevel) {
	mu.Lock()
	defer mu.Unlock()
	namedLevels = l

	var minLevel = logger.Level()
	for k, l := range namedLevels {
		g, err := glob.Compile(k)
		if err == nil {
			namedGlobs[k] = g
		}
		namedLevels[k] = l
		if l.Level() < minLevel {
			minLevel = l.Level()
		}
	}

	if minLevel < logger.Level() {
		// recreate logger if the min level is lower than the current min one
		loggerConfig.Level = zap.NewAtomicLevelAt(minLevel)
		logger, _ = loggerConfig.Build()
	}

	for name, nl := range namedLoggers {
		level := getLevel(name)
		// this can be racy, but
		nl.Logger = zap.New(logger.Core()).WithOptions(
			zap.IncreaseLevel(level),
		).Named(name)
	}
}

func Default() *zap.Logger {
	mu.Lock()
	defer mu.Unlock()
	return logger
}

func getLevel(name string) zap.AtomicLevel {
	level, ok := namedLevels[name]
	if !ok {
		var found bool
		for globName, glob := range namedGlobs {
			if glob.Match(name) {
				found = true
				level, _ = namedLevels[globName]
				// no need to check ok, because we know that globName exists
				break
			}
		}
		if !found {
			level = loggerConfig.Level
		}
	}
	return level
}

func NewNamed(name string, fields ...zap.Field) CtxLogger {
	mu.Lock()
	defer mu.Unlock()

	if l, nameExists := namedLoggers[name]; nameExists {
		return l
	}

	level := getLevel(name)
	l := zap.New(logger.Core()).WithOptions(
		zap.IncreaseLevel(level),
	).Named(name)

	ctxL := CtxLogger{l}
	namedLoggers[name] = ctxL
	return ctxL
}
